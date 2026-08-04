package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// A stand-in for the built dist/: a root index.html, a hashed asset, and an
// asset directory with no index of its own.
func testFS() http.FileSystem {
	return http.FS(fstest.MapFS{
		"index.html":            {Data: []byte("<!doctype html>")},
		"assets/app-abc123.js":  {Data: []byte("export const a = 1;")},
		"assets/app-abc123.css": {Data: []byte(":root{}")},
		"drafts/notes.txt":      {Data: []byte("unreferenced")},
	})
}

func get(t *testing.T, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	newHandler(testFS()).ServeHTTP(rec, req)
	return rec.Result()
}

func TestHealthProbe(t *testing.T) {
	res := get(t, "/health")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("/health = %d, want 200", res.StatusCode)
	}
}

func TestServesIndexAndAssets(t *testing.T) {
	for _, path := range []string{"/", "/assets/app-abc123.js"} {
		if res := get(t, path); res.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, res.StatusCode)
		}
	}
}

// A directory without an index.html must 404 rather than list its contents —
// unreferenced content in the build output should not be discoverable by
// browsing.
func TestDirectoryListingIsSuppressed(t *testing.T) {
	for _, path := range []string{"/assets/", "/drafts/"} {
		res := get(t, path)
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (directory listing leaked)", path, res.StatusCode)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	res := get(t, "/")
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	if got := res.Header.Get("Referrer-Policy"); got == "" {
		t.Error("Referrer-Policy header is missing")
	}
}
