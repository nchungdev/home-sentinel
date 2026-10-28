package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/nchungdev/home-sentinel/internal/pairing"
)

const (
	pairingTTL      = 5 * time.Minute
	pairingInterval = 2
	pairPagePath    = "/pair"
)

var tvPairings = pairing.NewStore(pairingTTL)

func (s *Server) handlePairStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	entry, err := tvPairings.Start()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "pairing_failed")
		return
	}
	writeJSON(w, map[string]any{
		"pair_id":     entry.ID,
		"code":        entry.Code,
		"approve_url": approveURL(r, entry.Code),
		"expires_in":  int(time.Until(entry.ExpiresAt).Seconds()),
		"interval":    pairingInterval,
	})
}

func (s *Server) handlePairPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	approved, found := tvPairings.Poll(r.URL.Query().Get("id"))
	if !found {
		writeJSONError(w, http.StatusNotFound, "expired")
		return
	}
	if !approved {
		writeJSON(w, map[string]any{"status": "pending"})
		return
	}
	writeJSON(w, map[string]any{"status": "approved", "token": s.Auth.Token()})
}

func (s *Server) handlePairApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	var payload struct {
		Code string `json:"code"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if !tvPairings.Approve(payload.Code) {
		writeJSONError(w, http.StatusNotFound, "unknown_code")
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func servePairPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(pairPage))
}

func approveURL(r *http.Request, code string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	link := url.URL{Scheme: scheme, Host: r.Host, Path: pairPagePath}
	link.RawQuery = url.Values{"code": {code}}.Encode()
	return link.String()
}
