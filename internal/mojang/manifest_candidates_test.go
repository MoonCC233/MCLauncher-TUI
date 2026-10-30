package mojang

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A mirror that answers 200 with an empty manifest must not win over the next
// candidate, otherwise the launcher treats it as "no versions at all".
func TestFetchManifestCandidatesSkipsEmptyManifests(t *testing.T) {
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"latest":{"release":"1.21.4","snapshot":"1.21.4"},"versions":[]}`))
	}))
	defer empty.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"latest":{"release":"1.21.4","snapshot":"1.21.4"},"versions":[{"id":"1.21.4","type":"release","url":"https://example.test/1.21.4.json"}]}`))
	}))
	defer good.Close()

	var raw rawManifest
	if err := fetchJSONCandidatesManifest(context.Background(), empty.Client(), []string{empty.URL, good.URL}, &raw); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(raw.Versions) != 1 || raw.Versions[0].ID != "1.21.4" {
		t.Fatalf("manifest = %+v, want the non-empty candidate", raw)
	}
}
