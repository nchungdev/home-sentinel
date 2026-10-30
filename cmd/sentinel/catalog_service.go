package main

import (
	"log"

	"github.com/nchungdev/home-sentinel/internal/cache"
	"github.com/nchungdev/home-sentinel/internal/catalog"
)

func (s *Server) catalog() *catalog.Service {
	s.catalogOnce.Do(func() {
		store, err := cache.FromEnvironment()
		if err != nil {
			log.Print("Invalid REDIS_URL; catalog uses bounded memory cache")
		}
		phimAPIBase := envDefault("PHIMAPI_URL", "https://phimapi.com")
		ophimBase := envDefault("OPHIM_URL", "https://ophim1.com")
		nguoncBase := envDefault("NGUONC_URL", "https://phim.nguonc.com")
		motchillBase := envDefault("MOTCHILL_URL", "https://motchill.tv")
		rophimBase := envDefault("ROPHIM_URL", "https://rophim.me")
		phimtuoithoBase := envDefault("PHIMTUOITHO_URL", "https://phimtuoitho.org")

		s.catalogService = &catalog.Service{
			Sources: map[string]catalog.Source{
				"phimapi":     catalog.NewPhimAPI(phimAPIBase),
				"ophim":       catalog.NewPhimAPI(ophimBase),
				"nguonc":      catalog.NewPhimAPI(nguoncBase),
				"motchill":    catalog.NewPhimAPI(motchillBase),
				"rophim":      catalog.NewPhimAPI(rophimBase),
				"phimtuoitho": catalog.NewPhimAPI(phimtuoithoBase),
			},
			Cache: store,
		}
	})

	return s.catalogService
}
