package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/nchungdev/home-sentinel/internal/requestflow"
)

type MovieMetadata struct {
	Token   string
	APIKey  string
	BaseURL string
	Client  *http.Client
}

func (s MovieMetadata) Execute(ctx context.Context, task requestflow.Task) requestflow.Result {
	result := requestflow.Result{Cache: "bypassed", Status: "skipped", Items: []requestflow.Item{}}
	if !s.hasCredentials() {
		result.ErrorCode = "not_configured"
		return result
	}
	payload, code := s.searchMetadata(ctx, task.Query)
	if code != "" {
		result.Status, result.ErrorCode = "error", code
		return result
	}
	result.Status = "empty"
	for _, entry := range payload.Results {
		if entry.MediaType != "movie" && entry.MediaType != "tv" {
			continue
		}
		date := entry.ReleaseDate
		if entry.MediaType == "tv" {
			date = entry.AirDate
		}
		title := entry.Title
		if entry.MediaType == "tv" {
			title = entry.Name
		}
		year := 0
		if len(date) >= 4 {
			year, _ = strconv.Atoi(date[:4])
		}
		identifier := entry.MediaType + "/" + strconv.Itoa(entry.ID)
		kind := "movie"
		if entry.MediaType == "tv" {
			kind = "show"
		}
		content := entry.Overview
		if task.Strategy != "tmdb_search" {
			content = s.description(ctx, identifier, entry.MediaType)
			if content == "" {
				content = entry.Overview
			}
		}
		actions := []requestflow.ItemAction{{Type: "media_detail",
			Label: "Chi tiết và nguồn phát", ItemID: identifier}}
		result.Items = append(result.Items, requestflow.Item{
			ID: "tmdb:" + identifier, Kind: kind, Title: title,
			URL: "https://www.themoviedb.org/" + identifier, Content: content,
			Poster: tmdbPoster(entry.PosterPath), Year: year,
			ExternalIDs: map[string]string{"tmdb": strconv.Itoa(entry.ID)},
			Actions:     actions,
		})
		if len(result.Items) == 8 {
			break
		}
	}
	if len(result.Items) > 0 {
		result.Status = "ok"
	}

	return result
}

type TMDBDetail struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Title       string            `json:"title"`
	Year        int               `json:"year,omitempty"`
	Overview    string            `json:"overview,omitempty"`
	Poster      string            `json:"poster,omitempty"`
	ExternalIDs map[string]string `json:"external_ids"`
	Aliases     []string          `json:"aliases,omitempty"`
}

type tmdbAlternativeTitle struct {
	Title   string `json:"title"`
	Name    string `json:"name"`
	Country string `json:"iso_3166_1"`
}

func (s MovieMetadata) Detail(ctx context.Context, mediaType string, id int) (TMDBDetail, string) {
	if !s.hasCredentials() {
		return TMDBDetail{}, "not_configured"
	}
	if id <= 0 || (mediaType != "movie" && mediaType != "tv") {
		return TMDBDetail{}, "invalid_media_id"
	}
	var payload struct {
		ID            int    `json:"id"`
		Title         string `json:"title"`
		Name          string `json:"name"`
		Overview      string `json:"overview"`
		OriginalTitle string `json:"original_title"`
		OriginalName  string `json:"original_name"`
		ReleaseDate   string `json:"release_date"`
		AirDate       string `json:"first_air_date"`
		PosterPath    string `json:"poster_path"`
		ExternalIDs   struct {
			IMDbID string `json:"imdb_id"`
			TVDBID int    `json:"tvdb_id"`
		} `json:"external_ids"`
		AlternativeTitles struct {
			Titles  []tmdbAlternativeTitle `json:"titles"`
			Results []tmdbAlternativeTitle `json:"results"`
		} `json:"alternative_titles"`
	}
	path := fmt.Sprintf("/%s/%d?language=vi-VN&append_to_response=external_ids,alternative_titles",
		mediaType, id)
	if code := s.read(ctx, path, &payload); code != "" {
		return TMDBDetail{}, code
	}
	if payload.ID != id {
		return TMDBDetail{}, "invalid_source_response"
	}
	title, kind, date := payload.Title, "movie", payload.ReleaseDate
	originalTitle := payload.OriginalTitle
	if mediaType == "tv" {
		title, kind, date = payload.Name, "show", payload.AirDate
		originalTitle = payload.OriginalName
	}
	if strings.TrimSpace(title) == "" {
		return TMDBDetail{}, "invalid_source_response"
	}
	year := 0
	if len(date) >= 4 {
		year, _ = strconv.Atoi(date[:4])
	}
	externalIDs := map[string]string{"tmdb": strconv.Itoa(id)}
	if payload.ExternalIDs.IMDbID != "" {
		externalIDs["imdb"] = payload.ExternalIDs.IMDbID
	}
	if payload.ExternalIDs.TVDBID > 0 {
		externalIDs["tvdb"] = strconv.Itoa(payload.ExternalIDs.TVDBID)
	}

	return TMDBDetail{ID: "tmdb:" + mediaType + "/" + strconv.Itoa(id), Kind: kind,
		Title: title, Year: year, Overview: payload.Overview,
		Poster: tmdbPoster(payload.PosterPath), ExternalIDs: externalIDs,
		Aliases: tmdbAliases(title, originalTitle, payload.AlternativeTitles.Titles,
			payload.AlternativeTitles.Results)}, ""
}

func tmdbAliases(
	title, originalTitle string,
	movieTitles, seriesTitles []tmdbAlternativeTitle,
) []string {
	alternatives := append(movieTitles, seriesTitles...)
	sort.SliceStable(alternatives, func(left, right int) bool {
		return tmdbAliasCountryRank(alternatives[left].Country) <
			tmdbAliasCountryRank(alternatives[right].Country)
	})
	titles := []string{title, originalTitle}
	for _, alternative := range alternatives {
		titles = append(titles, alternative.Title, alternative.Name)
	}

	seen := make(map[string]bool)
	unique := make([]string, 0, len(titles))
	for _, title := range titles {
		title = strings.TrimSpace(title)
		if title == "" || seen[title] {
			continue
		}
		seen[title] = true
		unique = append(unique, title)
	}

	return unique
}

func tmdbAliasCountryRank(country string) int {
	switch strings.ToUpper(country) {
	case "VN":
		return 0
	case "US", "GB":
		return 1
	default:
		return 2
	}
}

func tmdbPoster(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return ""
	}

	return "https://image.tmdb.org/t/p/w500" + path
}

func (s MovieMetadata) read(ctx context.Context, path string, output any) string {
	if !s.hasCredentials() || s.Client == nil {
		return "not_configured"
	}
	endpoint, err := url.Parse(s.BaseURL + path)
	if err != nil {
		return "not_configured"
	}
	if s.APIKey != "" {
		query := endpoint.Query()
		query.Set("api_key", s.APIKey)
		endpoint.RawQuery = query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "not_configured"
	}
	if s.Token != "" {
		request.Header.Set("Authorization", "Bearer "+s.Token)
	}
	response, err := s.Client.Do(request)
	if err != nil {
		return "provider_unavailable"
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return "auth_required"
	}
	if response.StatusCode != http.StatusOK {
		return "provider_unavailable"
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 || json.Unmarshal(data, output) != nil {
		return "invalid_source_response"
	}

	return ""
}

func (s MovieMetadata) hasCredentials() bool {
	return s.BaseURL != "" && (s.Token != "" || s.APIKey != "")
}
