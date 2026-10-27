package main

import (
	"net/http"

	"github.com/nchungdev/home-sentinel/agent/pkg/mediabridge"
	"github.com/nchungdev/home-sentinel/agent/pkg/transport"
	"github.com/nchungdev/home-sentinel/agent/pkg/watching"
)

const watchHistoryFile = "sentinel_watch_history.json"

func registerWatchRoutes(mux *http.ServeMux, cfg *transport.Config) {
	history := watching.NewHistory(watchHistoryFile)
	onDeck := watching.NewPlexOnDeck(mediabridge.NewBridge(""))
	lister := watching.NewContinue(history, onDeck)

	mux.Handle("/v2/watch/events", transport.NewV2WatchEventsHandler(history, cfg))
	mux.Handle("/v2/watch/continue", transport.NewV2WatchContinueHandler(lister, cfg))
}
