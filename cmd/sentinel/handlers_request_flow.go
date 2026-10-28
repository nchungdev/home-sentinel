package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/internal/requestflow"
)

func (s *Server) handleRequestFlow(w http.ResponseWriter, r *http.Request) {
	s.serveRequestFlow(w, r, requestflow.PlaceholderRunner(), false)
}

func (s *Server) handleLiveRequestFlow(w http.ResponseWriter, r *http.Request) {
	s.serveRequestFlow(w, r, s.searchRunner(), true)
}

func (s *Server) serveRequestFlow(
	w http.ResponseWriter, r *http.Request, runner requestflow.Runner, live bool,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input requestflow.Input
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" || len(input.Text) > 4000 || !requestflow.ValidContext(input.Context) {
		http.Error(w, "text must contain 1 to 4000 bytes", http.StatusBadRequest)
		return
	}
	if live {
		understanding, err := runner.Classifier.Understand(r.Context(), input)
		if err != nil {
			http.Error(w, "request cancelled", http.StatusRequestTimeout)
			return
		}
		plan := requestflow.BuildPlan(understanding)
		if !understanding.NeedsInput && !runnablePlan(plan, runner.Skills) {
			writeJSON(w, map[string]any{"status": "legacy_route"})
			return
		}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	encoder := json.NewEncoder(w)
	_ = runner.Run(ctx, input, func(event requestflow.Event) {
		if ctx.Err() != nil {
			return
		}
		if encoder.Encode(event) != nil {
			cancel()
			return
		}
		flusher.Flush()
	})
}
