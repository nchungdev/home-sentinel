package mediabridge

import "testing"

func TestCleanQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tìm phim avatar vietsub", "avatar"},
		{"xem anime naruto season 1", "naruto"},
		{"phim lẻ dune 2021", "dune 2021"},
	}

	for _, tc := range tests {
		actual := CleanQuery(tc.input)
		if actual != tc.expected {
			t.Fatalf("expected '%s', got '%s'", tc.expected, actual)
		}
	}
}
