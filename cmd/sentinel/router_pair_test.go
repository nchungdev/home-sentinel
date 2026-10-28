package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/adapters/nas/nastest"
)

type pairStart struct {
	PairID     string `json:"pair_id"`
	Code       string `json:"code"`
	ApproveURL string `json:"approve_url"`
}

type pairPoll struct {
	Status string `json:"status"`
	Token  string `json:"token"`
}

func TestTVPairingHandsOverTokenAfterApproval(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())

	response, payload := sentinel.request(t, nil, http.MethodPost, "/api/pair/start", "", nil)
	var started pairStart
	if response.StatusCode != 200 || json.Unmarshal(payload, &started) != nil {
		t.Fatalf("start: status %d %s", response.StatusCode, payload)
	}
	if !strings.Contains(started.ApproveURL, "/pair?code="+started.Code) {
		t.Fatalf("approve url %q lacks code %q", started.ApproveURL, started.Code)
	}

	pollPath := "/api/pair/poll?id=" + started.PairID
	if poll := pollPairing(t, sentinel, pollPath); poll.Status != "pending" || poll.Token != "" {
		t.Fatalf("want pending without token, got %+v", poll)
	}

	approve := map[string]string{"code": started.Code}
	if response, _ := sentinel.request(t, nil, http.MethodPost, "/api/pair/approve", "",
		approve); response.StatusCode != 401 {
		t.Fatalf("approve without login: want 401, got %d", response.StatusCode)
	}
	if response, _ := sentinel.request(t, nil, http.MethodPost, "/api/pair/approve",
		testTVToken, approve); response.StatusCode != 200 {
		t.Fatalf("approve: want 200, got %d", response.StatusCode)
	}

	if poll := pollPairing(t, sentinel, pollPath); poll.Status != "approved" ||
		poll.Token != testTVToken {
		t.Fatalf("want approved with token, got %+v", poll)
	}
	if code := sentinel.status(t, http.MethodGet, pollPath, ""); code != 404 {
		t.Fatalf("second poll: want 404, got %d", code)
	}
}

func TestPairPageIsPublic(t *testing.T) {
	sentinel := startSentinel(t, testTVToken, nastest.DefaultNAS())
	response, payload := sentinel.request(t, nil, http.MethodGet, "/pair?code=ABC234", "", nil)
	if response.StatusCode != 200 || !strings.Contains(string(payload), "Cho phép TV này") {
		t.Fatalf("pair page: status %d", response.StatusCode)
	}
}

func pollPairing(t *testing.T, sentinel *testSentinel, path string) pairPoll {
	t.Helper()
	response, payload := sentinel.request(t, nil, http.MethodGet, path, "", nil)
	var poll pairPoll
	if response.StatusCode != 200 || json.Unmarshal(payload, &poll) != nil {
		t.Fatalf("poll: status %d %s", response.StatusCode, payload)
	}
	return poll
}
