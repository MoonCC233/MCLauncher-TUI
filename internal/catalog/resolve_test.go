package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func versionWithFile(projectID, versionID, filename string) map[string]any {
	return map[string]any{
		"id":             versionID,
		"project_id":     projectID,
		"version_number": versionID,
		"game_versions":  []string{"1.21.1"},
		"loaders":        []string{"fabric"},
		"files": []map[string]any{{
			"filename": filename,
			"url":      "https://cdn.modrinth.com/data/" + projectID + "/" + filename,
			"size":     42,
			"primary":  true,
			"hashes":   map[string]string{"sha512": "sum-" + versionID},
		}},
	}
}

func TestResolveRequiredFollowsModrinthDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version/pinned":
			_ = json.NewEncoder(w).Encode(versionWithFile("a", "pinned", "a.jar"))
		case "/project/b/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{versionWithFile("b", "b1", "b.jar")})
		case "/project/d/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{versionWithFile("d", "d1", "d.jar")})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.BaseURL = server.URL
	client.FallbackBaseURL = ""

	root := Version{
		ID: "root1", ProjectID: "root",
		Dependencies: []Dependency{
			{ProjectID: "a", VersionID: "pinned", Type: "required"},
			{ProjectID: "b", Type: "required"},
			{ProjectID: "b", Type: "required"}, // duplicate project
			{ProjectID: "c", VersionID: "optional", Type: "optional"},
		},
	}
	// The dependency on a itself needs d.
	root.Files = []File{{Filename: "root.jar", URL: "https://cdn.modrinth.com/root.jar", Primary: true}}

	versions, err := client.ResolveRequired(context.Background(), SourceModrinth, root, "1.21.1", "fabric")
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool)
	for _, version := range versions {
		got[version.ProjectID] = true
	}
	for _, want := range []string{"a", "b"} {
		if !got[want] {
			t.Errorf("missing dependency %q in %+v", want, versions)
		}
	}
	if got["c"] {
		t.Error("optional dependencies must not be installed")
	}
	if len(versions) != 2 {
		t.Errorf("versions = %+v, want the duplicate to be collapsed", versions)
	}
	if file, ok := versions[0].PrimaryFile(); !ok || file.Filename != "a.jar" {
		t.Errorf("first dependency file = %+v", file)
	}
}

func TestResolveRequiredRecursesIntoDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version/pinned":
			payload := versionWithFile("a", "pinned", "a.jar")
			payload["dependencies"] = []map[string]any{{"project_id": "d", "dependency_type": "required"}}
			_ = json.NewEncoder(w).Encode(payload)
		case "/project/d/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{versionWithFile("d", "d1", "d.jar")})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.BaseURL = server.URL
	client.FallbackBaseURL = ""

	root := Version{ProjectID: "root", Dependencies: []Dependency{{ProjectID: "a", VersionID: "pinned", Type: "required"}}}
	versions, err := client.ResolveRequired(context.Background(), SourceModrinth, root, "1.21.1", "fabric")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("versions = %+v, want the nested dependency too", versions)
	}
	if versions[1].ProjectID != "d" {
		t.Errorf("nested dependency = %+v", versions[1])
	}
}

func TestResolveRequiredUsesTheNewestCurseForgeFile(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/mods/306612/files") {
			t.Errorf("path = %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		query = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
			"id": 99, "modId": 306612, "displayName": "Fabric API", "fileName": "fabric-api.jar",
			"downloadUrl":  "https://edge.forgecdn.net/files/0/99/fabric-api.jar",
			"fileLength":   1024,
			"hashes":       []map[string]any{{"value": "abc", "algo": 1}},
			"gameVersions": []string{"1.21.1", "Fabric"},
		}}})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.CurseForgeBaseURL = server.URL

	root := Version{ProjectID: "10", Dependencies: []Dependency{{ProjectID: "306612", Type: "required"}}}
	versions, err := client.ResolveRequired(context.Background(), SourceCurseForge, root, "1.21.1", "fabric")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].ProjectID != "306612" {
		t.Fatalf("versions = %+v", versions)
	}
	if !strings.Contains(query, "modLoaderType="+curseForgeLoaderFabric) || !strings.Contains(query, "gameVersion=1.21.1") {
		t.Errorf("query = %q, want the instance filters", query)
	}
}

func TestResolveRequiredIgnoresMissingDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.CurseForgeBaseURL = server.URL
	root := Version{ProjectID: "10", Dependencies: []Dependency{{ProjectID: "404", Type: "required"}}}
	versions, err := client.ResolveRequired(context.Background(), SourceCurseForge, root, "1.21.1", "fabric")
	if err != nil {
		t.Fatalf("a missing dependency must not fail the install: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("versions = %+v", versions)
	}
}
