package util

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
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

// A mirror that serves a corrupted file must not poison the install: the next
// URL is tried and the checksum is recomputed for it.
func TestDownloadFileFallsBackAfterChecksumMismatch(t *testing.T) {
	good := []byte("good payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mirror/file.jar" {
			_, _ = w.Write([]byte("corrupted"))
			return
		}
		_, _ = w.Write(good)
	}))
	defer server.Close()

	sum := sha512.Sum512(good)
	dest := filepath.Join(t.TempDir(), "file.jar")
	err := DownloadFile(context.Background(), server.Client(), DownloadSpec{
		URL:    server.URL + "/mirror/file.jar",
		Alt:    []string{server.URL + "/official/file.jar"},
		Dest:   dest,
		SHA512: hex.EncodeToString(sum[:]),
	}, nil)
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(good) {
		t.Errorf("content = %q, want the official payload", data)
	}
}
