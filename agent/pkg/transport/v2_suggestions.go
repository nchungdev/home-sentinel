package transport

import (
	"encoding/json"
	"net/http"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/proactive"
)

type V2SuggestionsHandler struct {
	engine *proactive.SuggestionEngine
	config *Config
}

func NewV2SuggestionsHandler(
	engine *proactive.SuggestionEngine,
	cfg *Config,
) *V2SuggestionsHandler {
	return &V2SuggestionsHandler{
		engine: engine,
		config: cfg,
	}
}

func (h *V2SuggestionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get(HeaderAuthorization)
	if !VerifyBearerToken(authHeader, h.config.Token) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	_, ctx, cancel := ExtractRequestContext(r, h.config.DefaultTimeout)
	defer cancel()

	snapshot := &contracts.ContextSnapshot{}

	suggestions, err := h.engine.GenerateSuggestions(ctx, snapshot)
	if err != nil {
		http.Error(w, `{"error":"suggestion_engine_failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set(HeaderContentType, "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	response := map[string]any{
		"suggestions": suggestions,
	}
	_ = json.NewEncoder(w).Encode(response)
}
