package llm

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCallbackOnlyRelaysMatchingLocalSession(t *testing.T) {
	requests := 0
	handler := func(w http.ResponseWriter, request *http.Request) {
		requests++
		if request.URL.Query().Get("code") != "test-code" {
			t.Error("callback code missing")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()
	endpoint := "https://claude.ai/oauth/authorize?state=test-state&redirect_uri="
	authURL := endpoint + url.QueryEscape(server.URL+"/callback")
	callback := server.URL + "/callback?state=test-state&code=test-code"
	if err := relayLoginCallback(authURL, callback); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		server.URL + "/other?state=test-state&code=test-code",
		server.URL + "/callback?state=wrong&code=test-code",
		"http://example.com/callback?state=test-state&code=test-code",
	} {
		if err := relayLoginCallback(authURL, invalid); err == nil {
			t.Fatal("unrelated callback accepted")
		}
	}
	if requests != 1 {
		t.Fatalf("relayed %d requests", requests)
	}
}
