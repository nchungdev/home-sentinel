package skills

import "testing"

func TestYoutubeThumbnail(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=xWeOf3f-n0w": "https://i.ytimg.com/vi/xWeOf3f-n0w/hqdefault.jpg",
		"https://youtu.be/xWeOf3f-n0w":                "https://i.ytimg.com/vi/xWeOf3f-n0w/hqdefault.jpg",
		"https://example.com/watch?v=xWeOf3f-n0w":     "",
		"https://www.youtube.com/results?q=naruto":    "",
	}
	for link, want := range cases {
		if got := youtubeThumbnail(link); got != want {
			t.Errorf("%s: want %q, got %q", link, want, got)
		}
	}
}
