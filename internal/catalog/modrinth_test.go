package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSearchPacksBuildsFacets(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %q, want /search", r.URL.Path)
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "mclauncher") {
			t.Errorf("User-Agent = %q, want it to identify the launcher", ua)
		}
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hits": []map[string]any{{
				"project_id": "abc123",
				"title":      "Fresh Textures",
				"downloads":  1234,
			}},
			"total_hits": 1,
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.BaseURL = server.URL
	result, err := client.SearchPacks(context.Background(), "fresh", "1.21.4", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].ID != "abc123" {
		t.Fatalf("hits = %+v", result.Hits)
	}
	if gotQuery.Get("query") != "fresh" {
		t.Errorf("query = %q", gotQuery.Get("query"))
	}
	facets := gotQuery.Get("facets")
	if !strings.Contains(facets, "project_type:resourcepack") || !strings.Contains(facets, "versions:1.21.4") {
		t.Errorf("facets = %q, want project type and game version", facets)
	}
	if gotQuery.Get("limit") != "20" || gotQuery.Get("index") != "relevance" {
		t.Errorf("query = %v", gotQuery)
	}
}

func TestSearchPacksWithoutGameVersion(t *testing.T) {
	var facets string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		facets = r.URL.Query().Get("facets")
		_ = json.NewEncoder(w).Encode(map[string]any{"hits": []any{}})
	}))
	defer server.Close()
	client := NewClient(server.Client())
	client.BaseURL = server.URL
	if _, err := client.SearchPacks(context.Background(), "", "", 20, 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(facets, "versions:") {
		t.Errorf("facets = %q, want no version restriction", facets)
	}
}

func TestVersionsFiltersFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/project/abc123/version" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if strings.Contains(r.URL.RawQuery, "game_versions") {
			t.Errorf("unexpected game_versions filter: %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id": "v1", "version_number": "1.0",
				"game_versions": []string{"1.21.4"},
				"files": []map[string]any{{
					"filename": "pack.zip", "url": "https://cdn.example/pack.zip",
					"size": 42, "primary": true,
					"hashes": map[string]string{"sha512": "deadbeef"},
				}},
			},
			{"id": "v2", "version_number": "2.0", "files": []map[string]any{}},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.BaseURL = server.URL
	versions, err := client.Versions(context.Background(), "abc123", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].ID != "v1" {
		t.Fatalf("versions = %+v, want only the one with files", versions)
	}
	file, ok := versions[0].PrimaryFile()
	if !ok || file.Filename != "pack.zip" || file.Hashes["sha512"] != "deadbeef" {
		t.Fatalf("file = %+v", file)
	}
}

func TestStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()
	client := NewClient(server.Client())
	client.BaseURL = server.URL
	if _, err := client.SearchPacks(context.Background(), "x", "", 20, 0); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}
