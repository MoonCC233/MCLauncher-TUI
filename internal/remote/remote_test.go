package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchCachedFallsBackToTheCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "issues.json")
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	cached := `[{"number":7,"title":"cached issue","state":"open","html_url":"https://example.test/7"}]`
	if err := os.WriteFile(cache, []byte(cached), 0o644); err != nil {
		t.Fatal(err)
	}
	// A year old cache is stale, but it is still better than nothing when every
	// source fails.
	old := time.Now().Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(cache, old, old); err != nil {
		t.Fatal(err)
	}

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer dead.Close()

	var list []Issue
	if err := fetchCached(context.Background(), dead.Client(), cache, false, &list, []string{dead.URL}); err != nil {
		t.Fatalf("stale cache should be used: %v", err)
	}
	if len(list) != 1 || list[0].Number != 7 {
		t.Fatalf("issues = %+v", list)
	}
}

func TestIssuesSkipPullRequests(t *testing.T) {
	body := `[
		{"number":1,"title":"a bug","state":"open","comments":3,"html_url":"https://example.test/1"},
		{"number":2,"title":"a pull request","state":"open","html_url":"https://example.test/2","pull_request":{}}
	]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	var list []Issue
	cache := filepath.Join(t.TempDir(), "issues.json")
	if err := fetchCached(context.Background(), server.Client(), cache, true, &list, []string{server.URL}); err != nil {
		t.Fatal(err)
	}
	issues := filterIssues(list)
	if len(issues) != 1 || issues[0].Number != 1 {
		t.Fatalf("issues = %+v, want only the bug", issues)
	}
}

func TestContributorsAndReleasesDecode(t *testing.T) {
	contributors := `[{"login":"MoonCC233","contributions":120,"html_url":"https://example.test/moon"}]`
	releases := `[{"tag_name":"v0.2.2","name":"v0.2.2","body":"notes","published_at":"2026-10-29T10:00:00Z","assets":[]}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/contributors" {
			_, _ = w.Write([]byte(contributors))
			return
		}
		_, _ = w.Write([]byte(releases))
	}))
	defer server.Close()

	var people []Contributor
	if err := fetchCached(context.Background(), server.Client(), filepath.Join(t.TempDir(), "c.json"), true, &people, []string{server.URL + "/contributors"}); err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 || people[0].Login != "MoonCC233" || people[0].Contributions != 120 {
		t.Fatalf("contributors = %+v", people)
	}
}

func TestReleaseNotesPreferTheRunningVersion(t *testing.T) {
	body := `[
		{"tag_name":"v0.3.0","name":"v0.3.0","body":"newest notes","published_at":"2026-11-01T00:00:00Z","assets":[]},
		{"tag_name":"v0.2.3","name":"v0.2.3","body":"notes for 0.2.3","published_at":"2026-10-30T00:00:00Z","assets":[]}
	]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "releases.json")
	// The running build is 0.2.3: its own notes win over the newer release.
	notes, err := fetchReleaseNotesFrom(context.Background(), server.Client(), cache, true, "0.2.3 (abc1234)", []string{server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].ID != "v0.2.3" {
		t.Fatalf("notes = %+v, want the v0.2.3 release", notes)
	}
	if notes[0].Body != "notes for 0.2.3" {
		t.Errorf("body = %q", notes[0].Body)
	}

	// A development build has no release of its own: show the newest one.
	dev, err := fetchReleaseNotesFrom(context.Background(), server.Client(), cache, true, "dev", []string{server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(dev) != 1 || dev[0].ID != "v0.3.0" {
		t.Fatalf("dev notes = %+v, want the newest release", dev)
	}
}

func TestSameVersionIgnoresTagPrefixAndCommit(t *testing.T) {
	cases := []struct {
		tag, current string
		want         bool
	}{
		{"v0.2.3", "0.2.3", true},
		{"v0.2.3", "0.2.3 (abc1234)", true},
		{"0.2.3", "0.2.3", true},
		{"v0.2.4", "0.2.3", false},
		{"v0.2.3", "dev", false},
	}
	for _, c := range cases {
		if got := sameVersion(c.tag, c.current); got != c.want {
			t.Errorf("sameVersion(%q, %q) = %v, want %v", c.tag, c.current, got, c.want)
		}
	}
}
