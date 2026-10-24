package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	apiPrefix         = "/api/v1"
	approvalCookie    = "freetv_approval"
	approvalCookieAge = 600
	maxBodyBytes      = 64 << 10
	bearerPrefix      = "Bearer "
	callbackPath      = "/auth/callback"
	pairPagePath      = "/pair"
	codeUnauthorized  = "unauthorized"
	codeUpstream      = "home_assistant_error"
	messageNotPaired  = "TV chưa được ghép đôi"
	messageRevoked    = "Phiên đã bị thu hồi"
)

type Caller struct {
	Key         string
	Session     Session
	AccessToken string
}

type Gateway struct {
	config        Config
	homeAssistant *HomeAssistant
	pairing       *PairingStore
	sessions      *SessionStore
}

type authedHandler func(http.ResponseWriter, *http.Request, Caller)

func NewGateway(config Config, homeAssistant *HomeAssistant, sessions *SessionStore) *Gateway {
	return &Gateway{
		config:        config,
		homeAssistant: homeAssistant,
		pairing:       NewPairingStore(time.Now),
		sessions:      sessions,
	}
}

func (gateway *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+apiPrefix+"/ping", gateway.handlePing)
	mux.HandleFunc("POST "+apiPrefix+"/pair/request", gateway.handlePairRequest)
	mux.HandleFunc("GET "+apiPrefix+"/pair/status", gateway.handlePairStatus)
	mux.HandleFunc("GET "+apiPrefix+"/devices", gateway.authed(gateway.handleDevices))
	mux.HandleFunc("GET "+apiPrefix+"/devices/{id}", gateway.authed(gateway.handleDevice))
	mux.HandleFunc("POST "+apiPrefix+"/devices/{id}/actions", gateway.authed(gateway.handleAction))
	mux.HandleFunc("GET "+apiPrefix+"/devices/{id}/stream", gateway.authed(gateway.handleStream))
	mux.HandleFunc("DELETE "+apiPrefix+"/session", gateway.authed(gateway.handleLogout))
	mux.HandleFunc("GET "+pairPagePath, gateway.handlePairPage)
	mux.HandleFunc("GET "+pairPagePath+"/approve", gateway.handleApprove)
	mux.HandleFunc("GET "+pairPagePath+"/reject", gateway.handleReject)
	mux.HandleFunc("GET "+callbackPath, gateway.handleCallback)
	return mux
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{
		"ok":    false,
		"error": map[string]string{"code": code, "message": message},
	})
}

func statusOfAction(code string) int {
	switch code {
	case errorNotFound:
		return http.StatusNotFound
	case errorUnsupported, errorInvalidValue, errorReadOnly:
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func readJSON(request *http.Request, into any) error {
	limited := http.MaxBytesReader(nil, request.Body, maxBodyBytes)
	return json.NewDecoder(limited).Decode(into)
}

func (gateway *Gateway) baseURL(request *http.Request) string {
	if gateway.config.GatewayURL != "" {
		return gateway.config.GatewayURL
	}

	scheme := "http"
	if request.TLS != nil || request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + request.Host
}

func (gateway *Gateway) authed(next authedHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		token := strings.TrimPrefix(request.Header.Get("Authorization"), bearerPrefix)
		key, session, found := gateway.sessions.Lookup(token)
		if token == "" || !found {
			writeError(writer, http.StatusUnauthorized, codeUnauthorized, messageNotPaired)
			return
		}

		accessToken, tokenErr := gateway.sessions.AccessToken(
			request.Context(), key, session, gateway.homeAssistant,
		)
		if errors.Is(tokenErr, errHomeAssistantUnauthorized) {
			_, _ = gateway.sessions.Remove(key)
			writeError(writer, http.StatusUnauthorized, codeUnauthorized, messageRevoked)
			return
		}
		if tokenErr != nil {
			writeError(writer, http.StatusBadGateway, codeUpstream, tokenErr.Error())
			return
		}

		next(writer, request, Caller{Key: key, Session: session, AccessToken: accessToken})
	}
}

func (gateway *Gateway) writeUpstreamError(writer http.ResponseWriter, upstreamErr error) {
	switch {
	case errors.Is(upstreamErr, errHomeAssistantUnauthorized):
		writeError(writer, http.StatusForbidden, "forbidden", "Home Assistant từ chối quyền")
	case errors.Is(upstreamErr, errHomeAssistantNotFound):
		writeError(writer, http.StatusNotFound, errorNotFound, "Không có thiết bị này")
	default:
		writeError(writer, http.StatusBadGateway, codeUpstream, upstreamErr.Error())
	}
}

func (gateway *Gateway) handlePing(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "service": "freetv-gateway"})
}

func (gateway *Gateway) handlePairRequest(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = readJSON(request, &body)

	pending, createErr := gateway.pairing.Create(body.Name)
	if createErr != nil {
		writeError(writer, http.StatusTooManyRequests, "too_many_requests", createErr.Error())
		return
	}

	approveURL := gateway.baseURL(request) + pairPagePath + "?request_id=" + pending.ID
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":          true,
		"request_id":  pending.ID,
		"code":        pending.Code,
		"approve_url": approveURL,
	})
}

func (gateway *Gateway) handlePairStatus(writer http.ResponseWriter, request *http.Request) {
	pending, collectErr := gateway.pairing.Collect(request.URL.Query().Get("request_id"))
	if collectErr != nil {
		writeError(writer, http.StatusNotFound, errorNotFound, collectErr.Error())
		return
	}

	body := map[string]any{"ok": true, "status": pending.Status}
	if pending.AccessToken != "" {
		body["access_token"] = pending.AccessToken
	}
	writeJSON(writer, http.StatusOK, body)
}

func buildDeviceList(states []HomeAssistantState, placements map[string]Placement) map[string]any {
	areas := map[string]string{}
	devices := []Device{}
	for _, state := range states {
		if !isSupported(state.EntityID) {
			continue
		}

		placement := placements[state.EntityID]
		device, describeErr := DescribeDevice(state.EntityID, state.State, state.Attributes)
		if describeErr != nil || placement.Hidden || len(device.Capabilities) == 0 {
			continue
		}

		device.Area = placement.AreaID
		devices = append(devices, device)
		if placement.AreaID != "" {
			areas[placement.AreaID] = placement.AreaName
		}
	}

	sort.Slice(devices, func(left, right int) bool {
		if devices[left].Area != devices[right].Area {
			return devices[left].Area < devices[right].Area
		}
		return strings.ToLower(devices[left].Name) < strings.ToLower(devices[right].Name)
	})
	return map[string]any{"version": 1, "areas": areaList(areas), "devices": devices}
}

func areaList(areas map[string]string) []map[string]string {
	listed := []map[string]string{}
	for id, name := range areas {
		listed = append(listed, map[string]string{"id": id, "name": name})
	}
	sort.Slice(listed, func(left, right int) bool {
		return listed[left]["name"] < listed[right]["name"]
	})
	return listed
}

func (gateway *Gateway) handleDevices(
	writer http.ResponseWriter,
	request *http.Request,
	caller Caller,
) {
	states, statesErr := gateway.homeAssistant.States(request.Context(), caller.AccessToken)
	if statesErr != nil {
		gateway.writeUpstreamError(writer, statesErr)
		return
	}

	placements := gateway.homeAssistant.Placements(request.Context(), caller.AccessToken)
	writeJSON(writer, http.StatusOK, buildDeviceList(states, placements))
}

func (gateway *Gateway) loadDevice(
	ctx context.Context,
	caller Caller,
	entityID string,
) (HomeAssistantState, error) {
	if !isSupported(entityID) {
		return HomeAssistantState{}, errHomeAssistantNotFound
	}
	return gateway.homeAssistant.State(ctx, caller.AccessToken, entityID)
}

func (gateway *Gateway) handleDevice(
	writer http.ResponseWriter,
	request *http.Request,
	caller Caller,
) {
	state, stateErr := gateway.loadDevice(request.Context(), caller, request.PathValue("id"))
	if stateErr != nil {
		gateway.writeUpstreamError(writer, stateErr)
		return
	}

	device, describeErr := DescribeDevice(state.EntityID, state.State, state.Attributes)
	if describeErr != nil {
		writeError(writer, http.StatusBadRequest, errorUnsupported, describeErr.Error())
		return
	}
	writeJSON(writer, http.StatusOK, device)
}

func (gateway *Gateway) handleAction(
	writer http.ResponseWriter,
	request *http.Request,
	caller Caller,
) {
	state, stateErr := gateway.loadDevice(request.Context(), caller, request.PathValue("id"))
	if stateErr != nil {
		gateway.writeUpstreamError(writer, stateErr)
		return
	}

	var body struct {
		Capability string `json:"capability"`
		Value      any    `json:"value"`
	}
	if readErr := readJSON(request, &body); readErr != nil || body.Capability == "" {
		writeError(writer, http.StatusBadRequest, errorInvalidValue, "cần capability và value")
		return
	}

	call, resolveErr := ResolveAction(state.EntityID, body.Capability, body.Value, state.Attributes)
	if resolveErr != nil {
		var failure *ActionError
		errors.As(resolveErr, &failure)
		writeError(writer, statusOfAction(failure.Code), failure.Code, failure.Message)
		return
	}

	if callErr := gateway.homeAssistant.CallService(
		request.Context(), caller.AccessToken, call,
	); callErr != nil {
		gateway.writeUpstreamError(writer, callErr)
		return
	}

	updated, updatedErr := gateway.homeAssistant.State(
		request.Context(), caller.AccessToken, state.EntityID,
	)
	if updatedErr != nil {
		gateway.writeUpstreamError(writer, updatedErr)
		return
	}

	device, _ := DescribeDevice(updated.EntityID, updated.State, updated.Attributes)
	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "device": device})
}

func (gateway *Gateway) handleStream(
	writer http.ResponseWriter,
	request *http.Request,
	caller Caller,
) {
	entityID := request.PathValue("id")
	if domainOf(entityID) != "camera" {
		writeError(writer, http.StatusNotFound, errorNotFound, "Không phải camera")
		return
	}

	gateway.homeAssistant.CameraStream(caller.AccessToken, entityID).ServeHTTP(writer, request)
}

func (gateway *Gateway) handleLogout(
	writer http.ResponseWriter,
	request *http.Request,
	caller Caller,
) {
	session, removeErr := gateway.sessions.Remove(caller.Key)
	if removeErr == nil {
		gateway.homeAssistant.Revoke(request.Context(), session.RefreshToken)
	}
	writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
}
