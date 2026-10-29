package skills

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

var mediaPartSuffix = regexp.MustCompile(
	`(?i)\s+(?:phần|mùa|season|part)\s+(?:\d+|[ivxlcdm]+)$`,
)

type tmdbSearchResponse struct {
	Results []struct {
		ID          int    `json:"id"`
		MediaType   string `json:"media_type"`
		Title       string `json:"title"`
		Name        string `json:"name"`
		Overview    string `json:"overview"`
		ReleaseDate string `json:"release_date"`
		AirDate     string `json:"first_air_date"`
		PosterPath  string `json:"poster_path"`
	} `json:"results"`
}

func (s MovieMetadata) searchMetadata(
	ctx context.Context,
	query string,
) (tmdbSearchResponse, string) {
	for _, candidate := range tmdbSearchQueries(query) {
		payload := tmdbSearchResponse{}
		for _, mediaType := range []string{"tv", "movie"} {
			path := "/search/" + mediaType + "?" + url.Values{
				"query": {candidate}, "language": {"vi-VN"}, "include_adult": {"false"},
			}.Encode()
			var typedResults tmdbSearchResponse
			if code := s.read(ctx, path, &typedResults); code != "" {
				return tmdbSearchResponse{}, code
			}
			for index := range typedResults.Results {
				typedResults.Results[index].MediaType = mediaType
			}
			payload.Results = append(payload.Results, typedResults.Results...)
		}
		if len(payload.Results) > 0 {
			return payload, ""
		}
	}

	return tmdbSearchResponse{}, ""
}

func tmdbSearchQueries(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	title := strings.TrimSpace(mediaPartSuffix.ReplaceAllString(query, ""))
	if title == "" || strings.EqualFold(title, query) {
		return []string{query}
	}

	return []string{title, query}
}
