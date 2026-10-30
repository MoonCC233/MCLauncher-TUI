package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// A mirror that answers 404 must not break the download: the official URL is
// tried next.
func TestDownloadFileFallsBackToAltURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mirror/file.jar" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "file.jar")
	err := DownloadFile(context.Background(), server.Client(), DownloadSpec{
		URL:  server.URL + "/mirror/file.jar",
		Alt:  []string{server.URL + "/official/file.jar"},
		Dest: dest,
	}, nil)
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Errorf("content = %q", data)
	}
}
