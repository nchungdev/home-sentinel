package catalog

import (
	"context"
	"net/url"
	"path"
	"strings"
)

func (p *PhimAPI) GetTitle(ctx context.Context, id string) (Title, error) {
	slug, err := titleSlug(id, "phimapi")
	if err != nil {
		return Title{}, err
	}
	detail, err := p.read(ctx, slug)
	if err != nil {
		return Title{}, err
	}
	title := Title{ID: id, SourceID: "phimapi", Name: detail.Movie.Name, Kind: "movie",
		Year: detail.Movie.Year, Seasons: []Season{}, Episodes: []Episode{}}
	seen := map[string]bool{}
	for _, group := range detail.Groups {
		for index, entry := range group.Episodes {
			if !slugPattern.MatchString(entry.Slug) || entry.Name == "" || len(group.Name) > 500 {
				return Title{}, ErrInvalidData
			}
			identifier := episodeID("phimapi", slug, group.Name, entry.Slug)
			if seen[identifier] {
				return Title{}, ErrInvalidData
			}
			seen[identifier] = true
			title.Episodes = append(title.Episodes, Episode{
				ID: identifier, TitleID: id, Label: entry.Name, Group: group.Name,
				Order: index + 1, Playable: supportedStream(entry.Stream),
			})
		}
	}
	if detail.Movie.Type == "series" || maxEpisodeCount(detail.Groups) > 1 {
		title.Kind = "series"
	}

	return title, nil
}

func (p *PhimAPI) ResolveEpisode(ctx context.Context, id string) (Stream, error) {
	parts, err := decodeEpisode(id, "phimapi")
	if err != nil {
		return Stream{}, err
	}
	detail, err := p.read(ctx, parts[0])
	if err != nil {
		return Stream{}, err
	}
	for _, group := range detail.Groups {
		if group.Name != parts[1] {
			continue
		}
		for index, entry := range group.Episodes {
			if entry.Slug != parts[2] {
				continue
			}
			if !supportedStream(entry.Stream) {
				return Stream{}, ErrUnsupported
			}
			return Stream{URL: entry.Stream, Title: detail.Movie.Name,
				Episode: Episode{ID: id, TitleID: "phimapi:" + parts[0], Label: entry.Name,
					Group: group.Name, Order: index + 1, Playable: true}}, nil
		}
	}

	return Stream{}, ErrNotFound
}

func supportedStream(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.User == nil && parsed.Hostname() != "" &&
		(parsed.Scheme == "https" || parsed.Scheme == "http") &&
		(strings.EqualFold(path.Ext(parsed.Path), ".m3u8") ||
			strings.EqualFold(path.Ext(parsed.Path), ".mp4"))
}

func maxEpisodeCount(groups []apiGroup) int {
	maximum := 0
	for _, group := range groups {
		if len(group.Episodes) > maximum {
			maximum = len(group.Episodes)
		}
	}

	return maximum
}
