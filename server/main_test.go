package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	newHandler(testFS(), nil).ServeHTTP(rec, req)
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

// Off-platform there is no Onklave identity: the config endpoint must 404 so
// the browser SDK stays off. On-platform it serves the browser-safe subset.
func TestOnklaveConfigEndpoint(t *testing.T) {
	if res := get(t, "/__onklave/config.json"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("config without identity = %d, want 404", res.StatusCode)
	}

	req := httptest.NewRequest(http.MethodGet, "/__onklave/config.json", nil)
	rec := httptest.NewRecorder()
	cfg := &onklaveBrowserConfig{ErrorsIngestKey: "oerr_live_x", Environment: "preview"}
	newHandler(testFS(), cfg).ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("config with identity = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	var body onklaveBrowserConfig
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ErrorsIngestKey != "oerr_live_x" || body.Environment != "preview" {
		t.Errorf("unexpected body: %+v", body)
	}
}

// The hybrid decrypt must open exactly what the platform produces:
// RSA-OAEP-SHA256(aesKey) : iv : AES-256-GCM(ct‖tag), base64, colon-joined.
func TestDecryptHybridRoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	aesKey := make([]byte, 32)
	iv := make([]byte, 12)
	if _, err := rand.Read(aesKey); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(iv); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte(`{"ONKLAVE_ERRORS_INGEST_KEY":"oerr_live_x"}`)

	encKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.PublicKey, aesKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, iv, plaintext, nil)

	payload := base64.StdEncoding.EncodeToString(encKey) + ":" +
		base64.StdEncoding.EncodeToString(iv) + ":" +
		base64.StdEncoding.EncodeToString(sealed)

	out, err := decryptHybrid(payload, key)
	if err != nil {
		t.Fatalf("decryptHybrid: %v", err)
	}
	if string(out) != string(plaintext) {
		t.Errorf("round trip mismatch: %q", out)
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
