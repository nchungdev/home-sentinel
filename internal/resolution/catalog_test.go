package resolution

import "testing"

func TestCatalogQueries(t *testing.T) {
	for _, query := range []string{"có phim nào k?", "có phim gì", "còn phim nào chưa xem"} {
		if !isCatalogQuery(query) {
			t.Errorf("%q should list the library", query)
		}
	}
	if isCatalogQuery("Inception") {
		t.Error("a title must stay a title search")
	}
}
