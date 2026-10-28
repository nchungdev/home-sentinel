package main

import (
	"net/http"

	"github.com/nchungdev/home-sentinel/internal/integrations/mediabridge"
	"github.com/nchungdev/home-sentinel/internal/transport"
	"github.com/nchungdev/home-sentinel/internal/watching"
)

const watchHistoryFile = "sentinel_watch_history.json"

func registerWatchRoutes(mux *http.ServeMux, cfg *transport.Config) {
	history := watching.NewHistory(watchHistoryFile)
	onDeck := watching.NewPlexOnDeck(mediabridge.NewBridge(""))
	lister := watching.NewContinue(history, onDeck)

	mux.Handle("/v2/watch/events", transport.NewV2WatchEventsHandler(history, cfg))
	mux.Handle("/v2/watch/continue", transport.NewV2WatchContinueHandler(lister, cfg))
}
