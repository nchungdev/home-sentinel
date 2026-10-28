package skills

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/nchungdev/home-sentinel/internal/requestflow"
)

type MovieMetadata struct {
	Token   string
	BaseURL string
	Client  *http.Client
}

func (s MovieMetadata) Execute(ctx context.Context, task requestflow.Task) requestflow.Result {
	result := requestflow.Result{Cache: "bypassed", Status: "skipped", Items: []requestflow.Item{}}
	if s.Token == "" {
		result.ErrorCode = "not_configured"
		return result
	}
	var payload struct {
		Results []struct {
			ID          int    `json:"id"`
			MediaType   string `json:"media_type"`
			Title       string `json:"title"`
			Name        string `json:"name"`
			Overview    string `json:"overview"`
			ReleaseDate string `json:"release_date"`
			AirDate     string `json:"first_air_date"`
		} `json:"results"`
	}
	path := "/search/multi?" + url.Values{
		"query": {task.Query}, "language": {"vi-VN"}, "include_adult": {"false"},
	}.Encode()
	if code := s.read(ctx, path, &payload); code != "" {
		result.Status, result.ErrorCode = "error", code
		return result
	}
	result.Status = "empty"
	for _, entry := range payload.Results {
		if entry.MediaType != "movie" && entry.MediaType != "tv" {
			continue
		}
		title, date := entry.Title, entry.ReleaseDate
		if entry.MediaType == "tv" {
			title, date = entry.Name, entry.AirDate
		}
		if len(date) >= 4 {
			title += " (" + date[:4] + ")"
		}
		identifier := entry.MediaType + "/" + strconv.Itoa(entry.ID)
		content := s.description(ctx, identifier, entry.MediaType)
		if content == "" {
			content = entry.Overview
		}
		result.Items = append(result.Items, requestflow.Item{
			ID: "tmdb:" + identifier, Kind: "media_title", Title: title,
			URL: "https://www.themoviedb.org/" + identifier, Content: content,
		})
		if len(result.Items) == 3 {
			break
		}
	}
	if len(result.Items) > 0 {
		result.Status = "ok"
	}

	return result
}

func (s MovieMetadata) read(ctx context.Context, path string, output any) string {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL+path, nil)
	if err != nil {
		return "not_configured"
	}
	request.Header.Set("Authorization", "Bearer "+s.Token)
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
