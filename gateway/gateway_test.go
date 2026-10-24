package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeHomeAssistant struct {
	mutex        sync.Mutex
	serviceCalls []string
	revoked      bool
}

func (fake *fakeHomeAssistant) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/token", func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		if request.Form.Get("grant_type") == "refresh_token" &&
			request.Form.Get("refresh_token") != "refresh-1" {
			http.Error(writer, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(writer,
			`{"access_token":"access-1","refresh_token":"refresh-1","expires_in":1800}`)
	})
	mux.HandleFunc("POST /auth/revoke", func(writer http.ResponseWriter, _ *http.Request) {
		fake.mutex.Lock()
		fake.revoked = true
		fake.mutex.Unlock()
	})
	mux.HandleFunc("GET /api/states", fake.authorized(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `[
			{"entity_id":"light.den_tran","state":"on","attributes":{"friendly_name":"Đèn trần",
			"supported_color_modes":["brightness"],"brightness":200}},
			{"entity_id":"vacuum.robot","state":"docked","attributes":{}},
			{"entity_id":"switch.hidden","state":"on","attributes":{}}]`)
	}))
	mux.HandleFunc("GET /api/states/{id}", fake.authorized(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"entity_id":"light.den_tran","state":"on","attributes":{
			"supported_color_modes":["brightness"],"brightness":120}}`)
	}))
	mux.HandleFunc("POST /api/template", fake.authorized(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `[
			{"id":"light.den_tran","area_id":"living_room","area_name":"Phòng khách","hidden":false},
			{"id":"switch.hidden","area_id":"","area_name":"","hidden":true}]`)
	}))
	mux.HandleFunc("POST /api/services/{domain}/{service}",
		fake.authorized(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			fake.mutex.Lock()
			fake.serviceCalls = append(fake.serviceCalls,
				request.PathValue("domain")+"."+request.PathValue("service")+" "+string(body))
			fake.mutex.Unlock()
			_, _ = io.WriteString(writer, "[]")
		}))
	return mux
}

func (fake *fakeHomeAssistant) authorized(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-1" {
			http.Error(writer, "no", http.StatusUnauthorized)
			return
		}
		next(writer, request)
	}
}

type gatewayFixture struct {
	t         *testing.T
	fake      *fakeHomeAssistant
	gateway   *httptest.Server
	hubClient *http.Client
}

func newGatewayFixture(t *testing.T) *gatewayFixture {
	t.Helper()

	fake := &fakeHomeAssistant{}
	homeAssistantServer := httptest.NewServer(fake.handler())
	t.Cleanup(homeAssistantServer.Close)

	config := Config{HomeAssistantURL: homeAssistantServer.URL}
	homeAssistant := NewHomeAssistant(homeAssistantServer.URL, homeAssistantServer.URL)
	sessions := NewSessionStore(filepath.Join(t.TempDir(), "sessions.json"), time.Now)
	gatewayServer := httptest.NewServer(NewGateway(config, homeAssistant, sessions).Handler())
	t.Cleanup(gatewayServer.Close)

	return &gatewayFixture{
		t:       t,
		fake:    fake,
		gateway: gatewayServer,
		hubClient: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
}

func (fixture *gatewayFixture) call(method, path, token, body string) (int, map[string]any) {
	fixture.t.Helper()

	request, _ := http.NewRequest(method, fixture.gateway.URL+path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, callErr := fixture.hubClient.Do(request)
	if callErr != nil {
		fixture.t.Fatalf("%s %s: %v", method, path, callErr)
	}
	defer response.Body.Close()

	decoded := map[string]any{}
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

func (fixture *gatewayFixture) pairTV() string {
	fixture.t.Helper()

	_, requested := fixture.call("POST", "/api/v1/pair/request", "", `{"name":"Living room TV"}`)
	requestID := requested["request_id"].(string)

	approve, approveErr := fixture.hubClient.Get(
		fixture.gateway.URL + "/pair/approve?request_id=" + url.QueryEscape(requestID),
	)
	if approveErr != nil {
		fixture.t.Fatalf("approve: %v", approveErr)
	}
	approve.Body.Close()
	redirect, _ := url.Parse(approve.Header.Get("Location"))
	nonce := redirect.Query().Get("state")
	if !strings.HasSuffix(redirect.Path, "/auth/authorize") || nonce == "" {
		fixture.t.Fatalf("approval must redirect to Home Assistant login, got %v", redirect)
	}

	callback, _ := http.NewRequest("GET",
		fixture.gateway.URL+"/auth/callback?code=abc&state="+nonce, nil)
	callback.AddCookie(approve.Cookies()[0])
	response, callbackErr := fixture.hubClient.Do(callback)
	if callbackErr != nil || response.StatusCode != http.StatusSeeOther {
		fixture.t.Fatalf("callback failed: %v %v", callbackErr, response)
	}
	response.Body.Close()

	_, status := fixture.call("GET", "/api/v1/pair/status?request_id="+requestID, "", "")
	token, _ := status["access_token"].(string)
	if status["status"] != pairingApproved || token == "" {
		fixture.t.Fatalf("expected approved with token, got %v", status)
	}
	return token
}

func TestDevicesRequireAPairedToken(t *testing.T) {
	fixture := newGatewayFixture(t)

	status, _ := fixture.call("GET", "/api/v1/devices", "", "")
	wrongStatus, _ := fixture.call("GET", "/api/v1/devices", "nope", "")

	if status != http.StatusUnauthorized || wrongStatus != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d and %d", status, wrongStatus)
	}
}

func TestPairedTVListsOnlySupportedVisibleDevicesWithAreas(t *testing.T) {
	fixture := newGatewayFixture(t)
	token := fixture.pairTV()

	status, list := fixture.call("GET", "/api/v1/devices", token, "")

	devices := list["devices"].([]any)
	if status != http.StatusOK || len(devices) != 1 {
		t.Fatalf("expected one device, got %d %v", status, list)
	}
	device := devices[0].(map[string]any)
	if device["id"] != "light.den_tran" || device["area"] != "living_room" {
		t.Fatalf("unexpected device %v", device)
	}
	areas := list["areas"].([]any)
	if len(areas) != 1 || areas[0].(map[string]any)["name"] != "Phòng khách" {
		t.Fatalf("unexpected areas %v", areas)
	}
}

func TestActionCallsHomeAssistantService(t *testing.T) {
	fixture := newGatewayFixture(t)
	token := fixture.pairTV()

	status, answer := fixture.call("POST", "/api/v1/devices/light.den_tran/actions", token,
		`{"capability":"brightness","value":120}`)

	if status != http.StatusOK || answer["ok"] != true {
		t.Fatalf("action failed: %d %v", status, answer)
	}
	if len(fixture.fake.serviceCalls) != 1 ||
		!strings.HasPrefix(fixture.fake.serviceCalls[0], "light.turn_on ") {
		t.Fatalf("unexpected service calls %v", fixture.fake.serviceCalls)
	}
}

func TestInvalidActionIsRejectedBeforeHomeAssistant(t *testing.T) {
	fixture := newGatewayFixture(t)
	token := fixture.pairTV()

	status, _ := fixture.call("POST", "/api/v1/devices/light.den_tran/actions", token,
		`{"capability":"brightness","value":999}`)

	if status != http.StatusBadRequest || len(fixture.fake.serviceCalls) != 0 {
		t.Fatalf("expected 400 without service call, got %d", status)
	}
}

func TestCallbackWithoutApprovalCookieIsRefused(t *testing.T) {
	fixture := newGatewayFixture(t)
	_, requested := fixture.call("POST", "/api/v1/pair/request", "", `{"name":"TV"}`)
	approve, _ := fixture.hubClient.Get(
		fixture.gateway.URL + "/pair/approve?request_id=" + requested["request_id"].(string),
	)
	approve.Body.Close()
	redirect, _ := url.Parse(approve.Header.Get("Location"))

	response, _ := fixture.hubClient.Get(
		fixture.gateway.URL + "/auth/callback?code=abc&state=" + redirect.Query().Get("state"),
	)
	response.Body.Close()
	_, status := fixture.call("GET", "/api/v1/pair/status?request_id="+requested["request_id"].(string),
		"", "")

	if status["status"] != pairingPending {
		t.Fatalf("pairing must stay pending without the cookie, got %v", status)
	}
}

func TestLogoutRevokesSessionAndHomeAssistantToken(t *testing.T) {
	fixture := newGatewayFixture(t)
	token := fixture.pairTV()

	fixture.call("DELETE", "/api/v1/session", token, "")
	status, _ := fixture.call("GET", "/api/v1/devices", token, "")

	if status != http.StatusUnauthorized || !fixture.fake.revoked {
		t.Fatalf("expected revoked session, got %d revoked=%v", status, fixture.fake.revoked)
	}
}
