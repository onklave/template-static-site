// Onklave platform wiring.
//
// On-platform, the pod carries a projected ServiceAccount token (mounted by
// the platform, audience `vault`) — its only credential. At startup this file
// uses it to fetch the app's per-environment secrets from vault, zero-trust:
// an ephemeral RSA key pair is generated, vault re-encrypts the secret map to
// it, and only this process can decrypt the response. From that map ONLY the
// browser-safe piece — the error-tracking ingest key — is kept, and served to
// the site's own pages at /__onklave/config.json so the front-end SDK
// (@onklave/errors/browser) can start. The key is rate-limited server-side
// and resolves org/project on the platform, never from the page.
//
// Off-platform (local dev, CI) there is no token file and all of this is a
// silent no-op: the endpoint 404s and the browser SDK stays off.
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultVaultURL  = "http://app-vault.app-vault.svc.cluster.local"
	defaultTokenPath = "/var/run/secrets/onklave/vault/vault-token"
	browserKeyName   = "ONKLAVE_ERRORS_INGEST_KEY"
)

// onklaveBrowserConfig is the browser-safe subset served to the site's pages.
type onklaveBrowserConfig struct {
	ErrorsIngestKey string `json:"errorsIngestKey"`
	Environment     string `json:"environment,omitempty"`
	Release         string `json:"release,omitempty"`
}

// loadOnklaveBrowserConfig fetches the pod's secrets and extracts the
// browser-safe config. Returns nil (never an error to the caller) when not
// running on Onklave or when anything about the fetch fails — the site must
// serve regardless.
func loadOnklaveBrowserConfig() *onklaveBrowserConfig {
	secrets, err := fetchOnklaveSecrets()
	if err != nil {
		// Expected off-platform (no token file). Log for on-platform debugging.
		fmt.Fprintf(os.Stderr, "onklave: browser config unavailable: %v\n", err)
		return nil
	}
	key := secrets[browserKeyName]
	if key == "" {
		return nil
	}
	return &onklaveBrowserConfig{
		ErrorsIngestKey: key,
		Environment:     os.Getenv("ONKLAVE_ENV"),
		Release:         os.Getenv("ONKLAVE_COMMIT_SHA"),
	}
}

// onklaveConfigHandler serves /__onklave/config.json. A nil config 404s.
func onklaveConfigHandler(cfg *onklaveBrowserConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if cfg == nil {
			http.NotFound(w, nil)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// The key can be rotated by a redeploy; don't let caches outlive it.
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(cfg)
	}
}

// fetchOnklaveSecrets mirrors @onklave/app-runtime's loadOnklaveSecrets:
// projected token as Bearer, ephemeral RSA-2048 consumer key (JWK), and a
// hybrid `RSA-OAEP-SHA256(aesKey):iv:AES-256-GCM(ct‖tag)` response payload
// (base64, colon-joined).
func fetchOnklaveSecrets() (map[string]string, error) {
	tokenPath := envOr("ONKLAVE_VAULT_TOKEN_PATH", defaultTokenPath)
	raw, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("no vault token: %w", err)
	}
	token := strings.TrimSpace(string(raw))

	orgID := os.Getenv("ONKLAVE_ORG_ID")
	projectID := os.Getenv("ONKLAVE_PROJECT_ID")
	environment := os.Getenv("ONKLAVE_ENV")
	if orgID == "" || projectID == "" || environment == "" {
		return nil, errors.New("ONKLAVE_ORG_ID / ONKLAVE_PROJECT_ID / ONKLAVE_ENV not set")
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]any{
		"organizationId":    orgID,
		"projectId":         projectID,
		"environment":       environment,
		"consumerPublicKey": rsaPublicJWK(&key.PublicKey),
	})
	if err != nil {
		return nil, err
	}

	vaultURL := strings.TrimRight(envOr("ONKLAVE_VAULT_URL", defaultVaultURL), "/")
	req, err := http.NewRequest(http.MethodPost, vaultURL+"/app-secrets/resolve", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("vault returned %d", res.StatusCode)
	}

	var payload struct {
		EncryptedValues string `json:"encryptedValues"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.EncryptedValues == "" {
		return nil, errors.New("vault returned no payload")
	}

	plaintext, err := decryptHybrid(payload.EncryptedValues, key)
	if err != nil {
		return nil, err
	}
	var secrets map[string]string
	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		return nil, err
	}
	return secrets, nil
}

// decryptHybrid opens `{b64 RSA-OAEP(aesKey)}:{b64 iv}:{b64 ct‖tag}`. Go's GCM
// expects the tag appended to the ciphertext, which is exactly how the payload
// arrives — no splitting needed.
func decryptHybrid(payload string, key *rsa.PrivateKey) ([]byte, error) {
	parts := strings.Split(payload, ":")
	if len(parts) != 3 {
		return nil, errors.New("malformed encrypted payload")
	}
	encKey, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	iv, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	sealed, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, key, encKey, nil)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, sealed, nil)
}

// rsaPublicJWK exports a public key in the JWK shape vault expects.
func rsaPublicJWK(pub *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
