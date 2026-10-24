package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestPairingApprovalIssuesWorkingToken(t *testing.T) {
	manager := NewPairingManager(filepath.Join(t.TempDir(), "paired.json"))

	pending, requestErr := manager.Request("Living room TV")
	if requestErr != nil {
		t.Fatalf("request failed: %v", requestErr)
	}

	status, token, _ := manager.Status(pending.ID)
	if status != pairingStatusPending || token != "" {
		t.Fatalf("expected pending without token, got %s %q", status, token)
	}

	if approveErr := manager.Approve(pending.ID); approveErr != nil {
		t.Fatalf("approve failed: %v", approveErr)
	}

	status, token, _ = manager.Status(pending.ID)
	if status != pairingStatusApproved || token == "" {
		t.Fatalf("expected approved with token, got %s %q", status, token)
	}
	if !manager.Authenticate(token) {
		t.Fatalf("issued token must authenticate")
	}

	if revokeErr := manager.Revoke(pending.ID); revokeErr != nil {
		t.Fatalf("revoke failed: %v", revokeErr)
	}
	if manager.Authenticate(token) {
		t.Fatalf("revoked token must not authenticate")
	}
}

func TestRequireRejectsUnpairedCallers(t *testing.T) {
	manager := NewPairingManager(filepath.Join(t.TempDir(), "paired.json"))
	reached := false
	guarded := manager.Require(func(http.ResponseWriter, *http.Request) { reached = true })

	recorder := httptest.NewRecorder()
	guarded(recorder, httptest.NewRequest(http.MethodGet, "/api/hub/overview", nil))
	if recorder.Code != http.StatusUnauthorized || reached {
		t.Fatalf("expected 401 without token, got %d", recorder.Code)
	}

	webRequest := httptest.NewRequest(http.MethodGet, "/api/hub/overview", nil)
	webRequest.Header.Set("Sec-Fetch-Site", "same-origin")
	guarded(httptest.NewRecorder(), webRequest)
	if !reached {
		t.Fatalf("hub web page must pass")
	}
}
