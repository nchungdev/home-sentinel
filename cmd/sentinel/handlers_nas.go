package main

import "net/http"

func (s *Server) handleNASStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	writeJSON(w, map[string]any{"ok": true, "nas": s.NAS.Snapshot(r.Context())})
}
