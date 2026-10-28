package main

import (
	"log"
	"os"

	"github.com/nchungdev/home-sentinel/internal/cache"
	"github.com/nchungdev/home-sentinel/internal/catalog"
)

func (s *Server) catalog() *catalog.Service {
	s.catalogOnce.Do(func() {
		store, err := cache.FromEnvironment()
		if err != nil {
			log.Print("Invalid REDIS_URL; catalog uses bounded memory cache")
		}
		base := os.Getenv("PHIMAPI_URL")
		if base == "" {
			base = "https://phimapi.com"
		}
		s.catalogService = &catalog.Service{
			Sources: map[string]catalog.Source{"phimapi": catalog.NewPhimAPI(base)}, Cache: store,
		}
	})

	return s.catalogService
}
