package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchCurseForgeMapsProjectsAndFilters(t *testing.T) {
	var query map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mods/search" {
			t.Errorf("path = %q, want /mods/search", r.URL.Path)
		}
		query = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"id":            238222,
				"slug":          "jei",
				"name":          "Just Enough Items",
				"summary":       "View items and recipes",
				"downloadCount": 12345,
				"authors":       []map[string]any{{"name": "mezz"}},
				"logo":          map[string]any{"url": "https://media.forgecdn.net/logo.png"},
			}},
			"pagination": map[string]any{"index": 0, "pageSize": 20, "resultCount": 1, "totalCount": 77},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.CurseForgeBaseURL = server.URL
	result, err := client.SearchSource(context.Background(), SourceCurseForge, ProjectTypeMod, "jei", "1.21.1", "fabric", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if query["classId"][0] != curseForgeClassMod {
		t.Errorf("classId = %q, want the mod class", query["classId"][0])
	}
	if query["searchFilter"][0] != "jei" || query["gameVersion"][0] != "1.21.1" {
		t.Errorf("query = %v, want the text and game version filters", query)
	}
	if query["modLoaderType"][0] != curseForgeLoaderFabric {
		t.Errorf("modLoaderType = %q, want fabric", query["modLoaderType"][0])
	}
	if query["sortOrder"][0] == "" {
		t.Error("sortOrder is required for the mirror to apply the search filter")
	}

	if len(result.Hits) != 1 {
		t.Fatalf("hits = %+v", result.Hits)
	}
	project := result.Hits[0]
	if project.ID != "238222" || project.Title != "Just Enough Items" || project.Author != "mezz" {
		t.Errorf("project = %+v", project)
	}
	if project.Source != SourceCurseForge {
		t.Errorf("source = %q, want CurseForge", project.Source)
	}
	if result.TotalHits != 77 {
		t.Errorf("total = %d, want 77", result.TotalHits)
	}
}

func TestSearchCurseForgeUsesTheRightClass(t *testing.T) {
	for projectType, want := range map[string]string{
		ProjectTypeResourcePack: curseForgeClassResourcePack,
		ProjectTypeShader:       curseForgeClassShader,
		ProjectTypeMod:          curseForgeClassMod,
	} {
		var classID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			classID = r.URL.Query().Get("classId")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "pagination": map[string]any{}})
		}))
		client := NewClient(server.Client())
		client.CurseForgeBaseURL = server.URL
		if _, err := client.SearchSource(context.Background(), SourceCurseForge, projectType, "", "", "", 20, 0); err != nil {
			t.Fatal(err)
		}
		if classID != want {
			t.Errorf("%s: classId = %q, want %q", projectType, classID, want)
		}
		server.Close()
	}
}

func TestCurseForgeVersionsMapsFilesAndDependencies(t *testing.T) {
	var query map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":            9379687,
					"modId":         238222,
					"displayName":   "JEI 19.70.0",
					"fileName":      "jei-1.21.1-fabric.jar",
					"downloadUrl":   "https://edge.forgecdn.net/files/9379/687/jei.jar",
					"fileLength":    1024,
					"fileDate":      "2026-10-31T01:58:38.193Z",
					"downloadCount": 15,
					"hashes":        []map[string]any{{"value": "abc123", "algo": 1}},
					"gameVersions":  []string{"1.21", "Client", "Fabric", "1.21.1"},
					"dependencies": []map[string]any{
						{"modId": 306612, "relationType": 3},
						{"modId": 1234, "relationType": 2},
					},
				},
				{
					"id":          1,
					"modId":       2,
					"fileName":    "not-distributable.jar",
					"downloadUrl": nil,
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.CurseForgeBaseURL = server.URL
	versions, err := client.VersionsForSource(context.Background(), SourceCurseForge, "238222", "1.21.1", "fabric")
	if err != nil {
		t.Fatal(err)
	}
	if query["gameVersion"][0] != "1.21.1" || query["modLoaderType"][0] != curseForgeLoaderFabric {
		t.Errorf("query = %v", query)
	}
	if len(versions) != 1 {
		t.Fatalf("versions = %+v, want only the distributable file", versions)
	}
	version := versions[0]
	if version.ID != "9379687" || version.ProjectID != "238222" || version.VersionNumber != "JEI 19.70.0" {
		t.Errorf("version = %+v", version)
	}
	file, ok := version.PrimaryFile()
	if !ok || file.Filename != "jei-1.21.1-fabric.jar" || file.Size != 1024 {
		t.Fatalf("file = %+v", file)
	}
	if file.Hashes["sha1"] != "abc123" {
		t.Errorf("hashes = %v", file.Hashes)
	}
	if len(version.Dependencies) != 1 || version.Dependencies[0].ProjectID != "306612" || version.Dependencies[0].Type != "required" {
		t.Errorf("dependencies = %+v, want only the required relation", version.Dependencies)
	}
	if got := version.GameVersions; len(got) != 2 || got[0] != "1.21" || got[1] != "1.21.1" {
		t.Errorf("game versions = %v, want the Minecraft versions only", got)
	}
	if got := version.Loaders; len(got) != 1 || got[0] != "fabric" {
		t.Errorf("loaders = %v", got)
	}
}

func TestCurseForgeLoaderID(t *testing.T) {
	cases := map[string]string{
		"forge":    curseForgeLoaderForge,
		"fabric":   curseForgeLoaderFabric,
		"quilt":    curseForgeLoaderQuilt,
		"neoforge": curseForgeLoaderNeoForge,
		"optifine": "",
		"":         "",
	}
	for loader, want := range cases {
		if got := curseForgeLoaderID(loader); got != want {
			t.Errorf("curseForgeLoaderID(%q) = %q, want %q", loader, got, want)
		}
	}
}

func TestSearchSortMapsToModrinthIndexes(t *testing.T) {
	for sort, want := range map[Sort]string{
		SortRelevance: "relevance",
		SortDownloads: "downloads",
		SortNewest:    "newest",
		SortUpdated:   "updated",
	} {
		var index string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			index = r.URL.Query().Get("index")
			_ = json.NewEncoder(w).Encode(map[string]any{"hits": []any{}})
		}))
		client := NewClient(server.Client())
		client.BaseURL = server.URL
		client.FallbackBaseURL = ""
		if _, err := client.SearchWithSort(context.Background(), ProjectTypeMod, "x", "", "", sort, 20, 0); err != nil {
			t.Fatal(err)
		}
		if index != want {
			t.Errorf("%s: index = %q, want %q", sort, index, want)
		}
		server.Close()
	}
}

func TestSearchSortMapsToCurseForgeSortFields(t *testing.T) {
	cases := map[Sort]string{
		SortRelevance: "",
		SortDownloads: "TotalDownloads",
		SortNewest:    "LastUpdated",
		SortUpdated:   "LastUpdated",
	}
	for sort, want := range cases {
		var field string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			field = r.URL.Query().Get("sortField")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "pagination": map[string]any{}})
		}))
		client := NewClient(server.Client())
		client.CurseForgeBaseURL = server.URL
		if _, err := client.SearchCatalog(context.Background(), SearchOptions{
			Source:      SourceCurseForge,
			ProjectType: ProjectTypeMod,
			Sort:        sort,
			Limit:       20,
		}); err != nil {
			t.Fatal(err)
		}
		if field != want {
			t.Errorf("%s: sortField = %q, want %q", sort, field, want)
		}
		server.Close()
	}
}
