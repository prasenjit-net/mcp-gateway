package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prasenjit-net/mcp-gateway/auth"
	"github.com/prasenjit-net/mcp-gateway/spec"
	"github.com/prasenjit-net/mcp-gateway/store"
)

// ── Encrypt / Decrypt ────────────────────────────────────────────────────────

func TestEncryptDecryptRoundtrip(t *testing.T) {
	secret := "test-secret-key"
	plaintext := []byte("super secret value")

	ct, err := auth.Encrypt(secret, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if string(ct) == string(plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}

	pt, err := auth.Decrypt(secret, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(pt) != string(plaintext) {
		t.Errorf("Decrypt = %q, want %q", pt, plaintext)
	}
}

func TestDecryptWrongSecret(t *testing.T) {
	ct, _ := auth.Encrypt("correct-secret", []byte("data"))
	_, err := auth.Decrypt("wrong-secret", ct)
	if err == nil {
		t.Error("expected error when decrypting with wrong secret")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	_, err := auth.Decrypt("secret", []byte("not-base64!!!"))
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	secret := "same-secret"
	pt := []byte("same plaintext")
	ct1, _ := auth.Encrypt(secret, pt)
	ct2, _ := auth.Encrypt(secret, pt)
	// Random nonce means each call should produce a different ciphertext.
	if string(ct1) == string(ct2) {
		t.Error("expected different ciphertexts on repeated encryption (random nonce)")
	}
}

// ── API Key ──────────────────────────────────────────────────────────────────

func TestAPIKeyApplyHeader(t *testing.T) {
	cfg := &store.AuthConfig{
		Type:   "api-key",
		Config: []byte(`{"header":"X-API-Key","value":"mykey"}`),
	}
	a, err := auth.NewAuthenticator(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("X-API-Key") != "mykey" {
		t.Errorf("X-API-Key = %q, want mykey", req.Header.Get("X-API-Key"))
	}
}

func TestAPIKeyApplyQuery(t *testing.T) {
	cfg := &store.AuthConfig{
		Type:   "api-key",
		Config: []byte(`{"query":"apikey","value":"qval"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req, _ := http.NewRequest("GET", "http://example.com/path", nil)
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	if req.URL.Query().Get("apikey") != "qval" {
		t.Errorf("query param apikey = %q, want qval", req.URL.Query().Get("apikey"))
	}
}

func TestAPIKeyDoesNotOverwriteExisting(t *testing.T) {
	cfg := &store.AuthConfig{
		Type:   "api-key",
		Config: []byte(`{"header":"X-API-Key","value":"newkey"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("X-API-Key", "existing")
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("X-API-Key") != "existing" {
		t.Errorf("header should not be overwritten, got %q", req.Header.Get("X-API-Key"))
	}
}

func TestAPIKeyWithEncryptedValue(t *testing.T) {
	secret := "gateway-secret"
	ct, _ := auth.Encrypt(secret, []byte("decrypted-key"))

	cfg := &store.AuthConfig{
		Type:   "api-key",
		Config: []byte(`{"header":"X-API-Key","value":"` + string(ct) + `"}`),
	}
	a, err := auth.NewAuthenticator(cfg, secret)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("X-API-Key") != "decrypted-key" {
		t.Errorf("X-API-Key = %q, want decrypted-key", req.Header.Get("X-API-Key"))
	}
}

// ── Bearer ───────────────────────────────────────────────────────────────────

func TestBearerApply(t *testing.T) {
	cfg := &store.AuthConfig{
		Type:   "bearer",
		Config: []byte(`{"token":"my-token"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "Bearer my-token" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
}

func TestBearerDoesNotOverwrite(t *testing.T) {
	cfg := &store.AuthConfig{
		Type:   "bearer",
		Config: []byte(`{"token":"new-token"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer existing")
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "Bearer existing" {
		t.Errorf("Authorization should not be overwritten")
	}
}

// ── Basic ────────────────────────────────────────────────────────────────────

func TestBasicApply(t *testing.T) {
	cfg := &store.AuthConfig{
		Type:   "basic",
		Config: []byte(`{"username":"user","password":"pass"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	u, p, ok := req.BasicAuth()
	if !ok {
		t.Fatal("BasicAuth() returned false")
	}
	if u != "user" || p != "pass" {
		t.Errorf("BasicAuth = (%q, %q), want (user, pass)", u, p)
	}
}

func TestBasicDoesNotOverwrite(t *testing.T) {
	cfg := &store.AuthConfig{
		Type:   "basic",
		Config: []byte(`{"username":"newuser","password":"newpass"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.SetBasicAuth("existing", "creds")
	orig := req.Header.Get("Authorization")
	if err := a.Apply(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != orig {
		t.Error("Authorization should not be overwritten")
	}
}

// ── NewAuthenticator — type dispatch ─────────────────────────────────────────

func TestNewAuthenticatorNone(t *testing.T) {
	a, err := auth.NewAuthenticator(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err != nil {
		t.Errorf("noop Apply returned error: %v", err)
	}
}

func TestNewAuthenticatorUnknownType(t *testing.T) {
	cfg := &store.AuthConfig{Type: "unknown", Config: []byte(`{}`)}
	a, err := auth.NewAuthenticator(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err != nil {
		t.Errorf("noop Apply returned error: %v", err)
	}
}

func TestNewAuthenticatorInvalidJSON(t *testing.T) {
	cfg := &store.AuthConfig{Type: "api-key", Config: []byte(`{invalid}`)}
	_, err := auth.NewAuthenticator(cfg, "")
	if err == nil {
		t.Error("expected error for invalid JSON config")
	}
}

// ── ApplyChain ───────────────────────────────────────────────────────────────

func TestApplyChainPassthroughAuth(t *testing.T) {
	tool := &spec.ToolDefinition{PassthroughAuth: true}
	inbound := &auth.InboundAuth{Authorization: "Bearer inbound-token"}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := auth.ApplyChain(req, inbound, tool, nil); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "Bearer inbound-token" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
}

func TestApplyChainPassthroughCookies(t *testing.T) {
	tool := &spec.ToolDefinition{PassthroughCookies: true}
	inbound := &auth.InboundAuth{Cookie: "session=abc"}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := auth.ApplyChain(req, inbound, tool, nil); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Cookie") != "session=abc" {
		t.Errorf("Cookie = %q", req.Header.Get("Cookie"))
	}
}

func TestApplyChainPassthroughDisabled(t *testing.T) {
	tool := &spec.ToolDefinition{PassthroughAuth: false}
	inbound := &auth.InboundAuth{Authorization: "Bearer secret"}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := auth.ApplyChain(req, inbound, tool, nil); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("Authorization should not be set when PassthroughAuth is false")
	}
}

func TestApplyChainConfiguredAuthApplied(t *testing.T) {
	tool := &spec.ToolDefinition{}
	cfg := &store.AuthConfig{
		Type:   "bearer",
		Config: []byte(`{"token":"configured-token"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := auth.ApplyChain(req, nil, tool, a); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "Bearer configured-token" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
}

// ── OAuth2 ───────────────────────────────────────────────────────────────────

func TestOAuth2ApplyFetchesToken(t *testing.T) {
	// Mock token endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"test-token","expires_in":3600}`))
	}))
	defer srv.Close()

	cfg := &store.AuthConfig{
		Type:   "oauth2",
		Config: []byte(`{"token_url":"` + srv.URL + `","client_id":"id","client_secret":"sec"}`),
	}
	a, err := auth.NewAuthenticator(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if req.Header.Get("Authorization") != "Bearer test-token" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
}

func TestOAuth2TokenCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"cached-token","expires_in":3600}`))
	}))
	defer srv.Close()

	cfg := &store.AuthConfig{
		Type:   "oauth2",
		Config: []byte(`{"token_url":"` + srv.URL + `","client_id":"id","client_secret":"sec"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req1, _ := http.NewRequest("GET", "http://example.com", nil)
	req2, _ := http.NewRequest("GET", "http://example.com", nil)
	a.Apply(req1) //nolint:errcheck
	a.Apply(req2) //nolint:errcheck
	if calls != 1 {
		t.Errorf("token endpoint called %d times, want 1 (should be cached)", calls)
	}
}

func TestOAuth2TokenEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := &store.AuthConfig{
		Type:   "oauth2",
		Config: []byte(`{"token_url":"` + srv.URL + `","client_id":"id","client_secret":"sec"}`),
	}
	a, _ := auth.NewAuthenticator(cfg, "")
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := a.Apply(req); err == nil {
		t.Error("expected error when token endpoint returns 400")
	}
}

func TestOAuth2DefaultExpiry(t *testing.T) {
// Token endpoint returns no expires_in — should use conservative default (300 s).
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")
w.Write([]byte(`{"access_token":"tok-default-expiry"}`))
}))
defer srv.Close()

cfg := &store.AuthConfig{
Type:   "oauth2",
Config: []byte(`{"token_url":"` + srv.URL + `","client_id":"cid","client_secret":"csec"}`),
}
a, err := auth.NewAuthenticator(cfg, "")
if err != nil {
t.Fatalf("NewAuthenticator: %v", err)
}
req, _ := http.NewRequest("GET", "http://example.com", nil)
if err := a.Apply(req); err != nil {
t.Fatalf("Apply: %v", err)
}
if req.Header.Get("Authorization") != "Bearer tok-default-expiry" {
t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
}
}
