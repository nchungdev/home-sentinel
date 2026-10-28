package skills

import (
	"net/url"
	"regexp"
	"strings"
)

var youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func valueOf(number *int) int {
	if number == nil {
		return 0
	}
	return *number
}

func youtubeThumbnail(link string) string {
	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(parsed.Hostname(), "www.")
	id := ""
	switch host {
	case "youtube.com", "m.youtube.com":
		id = parsed.Query().Get("v")
	case "youtu.be":
		id = strings.Trim(parsed.Path, "/")
	}
	if !youtubeID.MatchString(id) {
		return ""
	}
	return "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"
}
