package mediasearch

func (s *httpSource) Configured() bool {
	return s.baseURL != "" && (s.keyHeader == "" || s.apiKey != "")
}

func (s *youtubeSource) Configured() bool { return s.apiKey != "" }

func (s *Service) Configured(sourceID string) bool {
	for _, source := range s.sources {
		if source.Info().ID != sourceID {
			continue
		}
		configured, ok := source.(interface{ Configured() bool })
		return ok && configured.Configured()
	}

	return false
}
