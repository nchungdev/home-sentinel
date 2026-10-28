package catalog

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}_-]{0,199}$`)

func titleSlug(id, source string) (string, error) {
	prefix := source + ":"
	if !strings.HasPrefix(id, prefix) {
		return "", ErrInvalidID
	}
	slug := strings.TrimPrefix(id, prefix)
	if !slugPattern.MatchString(slug) {
		return "", ErrInvalidID
	}

	return slug, nil
}

func episodeID(source, title, group, slug string) string {
	serialized, _ := json.Marshal([]string{title, group, slug})
	return source + ":" + base64.RawURLEncoding.EncodeToString(serialized)
}

func decodeEpisode(id, source string) ([]string, error) {
	prefix := source + ":"
	if len(id) > 2048 || !strings.HasPrefix(id, prefix) {
		return nil, ErrInvalidID
	}
	serialized, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, prefix))
	var parts []string
	if err != nil || json.Unmarshal(serialized, &parts) != nil || len(parts) != 3 {
		return nil, ErrInvalidID
	}
	if !slugPattern.MatchString(parts[0]) || !slugPattern.MatchString(parts[2]) ||
		len(parts[1]) > 500 {
		return nil, ErrInvalidID
	}

	return parts, nil
}
