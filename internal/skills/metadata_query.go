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

type tmdbItem struct {
	ID          int    `json:"id"`
	MediaType   string `json:"media_type"`
	Title       string `json:"title"`
	Name        string `json:"name"`
	Overview    string `json:"overview"`
	ReleaseDate string `json:"release_date"`
	AirDate     string `json:"first_air_date"`
	PosterPath  string `json:"poster_path"`
}

type tmdbSearchResponse struct {
	Results []tmdbItem `json:"results"`
}

func (s MovieMetadata) searchMetadata(
	ctx context.Context,
	query string,
) (tmdbSearchResponse, string) {
	for _, candidate := range tmdbSearchQueries(query) {
		movieResults := tmdbSearchResponse{}
		tvResults := tmdbSearchResponse{}

		for _, mediaType := range []string{"movie", "tv"} {
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
			if mediaType == "movie" {
				movieResults = typedResults
			} else {
				tvResults = typedResults
			}
		}

		combined := interleaveTMDBResults(movieResults.Results, tvResults.Results)
		if len(combined) > 0 {
			return tmdbSearchResponse{Results: combined}, ""
		}
	}

	return tmdbSearchResponse{}, ""
}

func interleaveTMDBResults(movies, tvs []tmdbItem) []tmdbItem {
	combined := make([]tmdbItem, 0, len(movies)+len(tvs))
	maxLen := len(movies)
	if len(tvs) > maxLen {
		maxLen = len(tvs)
	}
	for i := 0; i < maxLen; i++ {
		if i < len(movies) {
			combined = append(combined, movies[i])
		}
		if i < len(tvs) {
			combined = append(combined, tvs[i])
		}
	}
	return combined
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
