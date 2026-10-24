package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const (
	homeAssistantTimeout = 10 * time.Second
	maxResponseBytes     = 8 << 20
	placementTemplate    = `{% set ns = namespace(items=[]) %}{% for s in states %}` +
		`{% set ns.items = ns.items + [{"id": s.entity_id, "area_id": area_id(s.entity_id), ` +
		`"area_name": area_name(s.entity_id), "hidden": is_hidden_entity(s.entity_id)}] %}` +
		`{% endfor %}{{ ns.items | tojson }}`
)

var (
	errHomeAssistantUnauthorized = errors.New("Home Assistant rejected the token")
	errHomeAssistantNotFound     = errors.New("Home Assistant has no such entity")
	errHomeAssistantBadRequest   = errors.New("Home Assistant refused the request")
)

type HomeAssistant struct {
	baseURL   string
	publicURL string
	client    *http.Client
}

type HomeAssistantState struct {
	EntityID   string     `json:"entity_id"`
	State      string     `json:"state"`
	Attributes Attributes `json:"attributes"`
}

type Placement struct {
	ID       string `json:"id"`
	AreaID   string `json:"area_id"`
	AreaName string `json:"area_name"`
	Hidden   bool   `json:"hidden"`
}

type TokenGrant struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func NewHomeAssistant(baseURL, publicURL string) *HomeAssistant {
	return &HomeAssistant{
		baseURL:   baseURL,
		publicURL: publicURL,
		client:    &http.Client{Timeout: homeAssistantTimeout},
	}
}

func (homeAssistant *HomeAssistant) AuthorizeURL(clientID, redirectURI, state string) string {
	query := url.Values{}
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	return homeAssistant.publicURL + "/auth/authorize?" + query.Encode()
}

func (homeAssistant *HomeAssistant) send(
	ctx context.Context,
	method, path, token, contentType string,
	body io.Reader,
	into any,
) error {
	request, requestErr := http.NewRequestWithContext(ctx, method, homeAssistant.baseURL+path, body)
	if requestErr != nil {
		return requestErr
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	response, responseErr := homeAssistant.client.Do(request)
	if responseErr != nil {
		return responseErr
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusUnauthorized:
		return errHomeAssistantUnauthorized
	case response.StatusCode == http.StatusNotFound:
		return errHomeAssistantNotFound
	case response.StatusCode == http.StatusBadRequest:
		return errHomeAssistantBadRequest
	case response.StatusCode >= 400:
		return fmt.Errorf("Home Assistant answered %d", response.StatusCode)
	}
	if into == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(into)
}

func (homeAssistant *HomeAssistant) States(
	ctx context.Context,
	token string,
) ([]HomeAssistantState, error) {
	var states []HomeAssistantState
	sendErr := homeAssistant.send(ctx, http.MethodGet, "/api/states", token, "", nil, &states)
	return states, sendErr
}

func (homeAssistant *HomeAssistant) State(
	ctx context.Context,
	token, entityID string,
) (HomeAssistantState, error) {
	var state HomeAssistantState
	path := "/api/states/" + url.PathEscape(entityID)
	sendErr := homeAssistant.send(ctx, http.MethodGet, path, token, "", nil, &state)
	return state, sendErr
}

func (homeAssistant *HomeAssistant) CallService(
	ctx context.Context,
	token string,
	call ServiceCall,
) error {
	payload, marshalErr := json.Marshal(call.Data)
	if marshalErr != nil {
		return marshalErr
	}

	path := "/api/services/" + url.PathEscape(call.Domain) + "/" + url.PathEscape(call.Service)
	return homeAssistant.send(
		ctx, http.MethodPost, path, token, "application/json", bytes.NewReader(payload), nil,
	)
}

func (homeAssistant *HomeAssistant) Placements(
	ctx context.Context,
	token string,
) map[string]Placement {
	payload, _ := json.Marshal(map[string]string{"template": placementTemplate})
	var placements []Placement
	sendErr := homeAssistant.send(
		ctx, http.MethodPost, "/api/template", token, "application/json",
		bytes.NewReader(payload), &placements,
	)
	byID := map[string]Placement{}
	if sendErr != nil {
		return byID
	}
	for _, placement := range placements {
		byID[placement.ID] = placement
	}
	return byID
}

func (homeAssistant *HomeAssistant) exchange(
	ctx context.Context,
	form url.Values,
) (TokenGrant, error) {
	var grant TokenGrant
	sendErr := homeAssistant.send(
		ctx, http.MethodPost, "/auth/token", "", "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()), &grant,
	)
	return grant, sendErr
}

func (homeAssistant *HomeAssistant) ExchangeCode(
	ctx context.Context,
	code, clientID string,
) (TokenGrant, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", clientID)
	return homeAssistant.exchange(ctx, form)
}

func (homeAssistant *HomeAssistant) RefreshAccess(
	ctx context.Context,
	refreshToken, clientID string,
) (TokenGrant, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	grant, exchangeErr := homeAssistant.exchange(ctx, form)
	rejected := errors.Is(exchangeErr, errHomeAssistantUnauthorized) ||
		errors.Is(exchangeErr, errHomeAssistantBadRequest)
	if exchangeErr != nil && !rejected {
		return TokenGrant{}, exchangeErr
	}
	if grant.AccessToken == "" {
		return TokenGrant{}, errHomeAssistantUnauthorized
	}
	return grant, nil
}

func (homeAssistant *HomeAssistant) Revoke(ctx context.Context, refreshToken string) {
	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("action", "revoke")
	_ = homeAssistant.send(
		ctx, http.MethodPost, "/auth/revoke", "", "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()), nil,
	)
}

func (homeAssistant *HomeAssistant) CameraStream(token, entityID string) *httputil.ReverseProxy {
	target, _ := url.Parse(homeAssistant.baseURL)
	return &httputil.ReverseProxy{
		FlushInterval: -1,
		Director: func(request *http.Request) {
			request.URL.Scheme = target.Scheme
			request.URL.Host = target.Host
			request.URL.Path = "/api/camera_proxy_stream/" + entityID
			request.URL.RawQuery = ""
			request.Host = target.Host
			request.Header.Set("Authorization", "Bearer "+token)
		},
	}
}
