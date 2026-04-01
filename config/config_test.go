package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prasenjit-net/mcp-gateway/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", cfg.DataDir)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.MaxResponseBytes != 1048576 {
		t.Errorf("MaxResponseBytes = %d, want 1048576", cfg.MaxResponseBytes)
	}
	if cfg.OpenAIModel != "gpt-4o" {
		t.Errorf("OpenAIModel = %q, want gpt-4o", cfg.OpenAIModel)
	}
	if cfg.AdminSessionTTL != 24 {
		t.Errorf("AdminSessionTTL = %d, want 24", cfg.AdminSessionTTL)
	}
	if len(cfg.CORS.AllowedOrigins) != 0 {
		t.Errorf("CORS.AllowedOrigins = %v, want empty", cfg.CORS.AllowedOrigins)
	}
	if len(cfg.CORS.AllowedMethods) == 0 {
		t.Error("CORS.AllowedMethods should not be empty")
	}
	if len(cfg.CORS.AllowedHeaders) == 0 {
		t.Error("CORS.AllowedHeaders should not be empty")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := config.Load("/tmp/does-not-exist-mcp-gateway-test.toml")
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}
	// Should return defaults.
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
}

func TestLoadTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
listen_addr = ":9090"
data_dir = "/tmp/test-data"
log_level = "debug"
max_response_bytes = 2097152
admin_password = "secret123"
admin_session_ttl_hours = 48

[cors]
allowed_origins = ["https://example.com", "https://app.example.com"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.DataDir != "/tmp/test-data" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.MaxResponseBytes != 2097152 {
		t.Errorf("MaxResponseBytes = %d", cfg.MaxResponseBytes)
	}
	if cfg.AdminPassword != "secret123" {
		t.Errorf("AdminPassword = %q", cfg.AdminPassword)
	}
	if cfg.AdminSessionTTL != 48 {
		t.Errorf("AdminSessionTTL = %d", cfg.AdminSessionTTL)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Errorf("CORS.AllowedOrigins = %v, want 2 entries", cfg.CORS.AllowedOrigins)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":7070")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("GATEWAY_SECRET", "mysecret")
	t.Setenv("ADMIN_PASSWORD", "adminpass")
	t.Setenv("MAX_RESPONSE_BYTES", "512000")

	cfg, err := config.Load("/tmp/does-not-exist.toml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":7070" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.GatewaySecret != "mysecret" {
		t.Errorf("GatewaySecret = %q", cfg.GatewaySecret)
	}
	if cfg.AdminPassword != "adminpass" {
		t.Errorf("AdminPassword = %q", cfg.AdminPassword)
	}
	if cfg.MaxResponseBytes != 512000 {
		t.Errorf("MaxResponseBytes = %d", cfg.MaxResponseBytes)
	}
}

func TestCORSDefaultsAppliedAfterLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// Config without [cors] section: methods/headers should get defaults.
	if err := os.WriteFile(path, []byte(`listen_addr = ":8080"`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CORS.AllowedMethods) == 0 {
		t.Error("AllowedMethods should have defaults")
	}
	if len(cfg.CORS.AllowedHeaders) == 0 {
		t.Error("AllowedHeaders should have defaults")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte(":::invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestTLSDefaultPaths(t *testing.T) {
	cfg, err := config.Load("/tmp/no-file.toml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TLS.CertFile == "" {
		t.Error("TLS.CertFile should have a default")
	}
	if cfg.TLS.KeyFile == "" {
		t.Error("TLS.KeyFile should have a default")
	}
}

// Regression test: openai_api_key and openai_model are root-level keys and
// MUST appear before any [section] header in config.toml. If they're placed
// after [cors], TOML silently parses them as cors.openai_api_key which is
// an unknown field and gets dropped, leaving cfg.OpenAIAPIKey empty.
func TestOpenAIKeyBeforeCORSSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Correct layout: openai_* before [cors]
	correct := `
openai_api_key = "sk-test-correct"
openai_model   = "gpt-4o-mini"

[cors]
allowed_origins = []
`
	if err := os.WriteFile(path, []byte(correct), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.OpenAIAPIKey != "sk-test-correct" {
		t.Errorf("OpenAIAPIKey = %q, want sk-test-correct (key placed before [cors])", cfg.OpenAIAPIKey)
	}
	if cfg.OpenAIModel != "gpt-4o-mini" {
		t.Errorf("OpenAIModel = %q, want gpt-4o-mini", cfg.OpenAIModel)
	}
}

// Ensure the built-in DefaultConfigContent template is parseable and produces
// a config with the expected openai_model (key is commented out in the template).
func TestDefaultConfigContentParseable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.toml")
	if err := os.WriteFile(path, []byte(config.DefaultConfigContent), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("DefaultConfigContent is not valid TOML: %v", err)
	}
	// Template has openai_model = "gpt-4o" before [cors].
	if cfg.OpenAIModel != "gpt-4o" {
		t.Errorf("DefaultConfigContent OpenAIModel = %q, want gpt-4o", cfg.OpenAIModel)
	}
	// Key is commented out in the template, so should be empty.
	if cfg.OpenAIAPIKey != "" {
		t.Errorf("DefaultConfigContent should not have a pre-set OpenAIAPIKey")
	}
}

func TestGatewaySecretFile(t *testing.T) {
dir := t.TempDir()
secretFile := filepath.Join(dir, "gateway_secret")
if err := os.WriteFile(secretFile, []byte("  my-secret-value\n"), 0o600); err != nil {
t.Fatal(err)
}
cfgFile := filepath.Join(dir, "config.toml")
if err := os.WriteFile(cfgFile, []byte(`gateway_secret_file = "`+secretFile+`"`), 0o644); err != nil {
t.Fatal(err)
}
cfg, err := config.Load(cfgFile)
if err != nil {
t.Fatal(err)
}
if cfg.GatewaySecret != "my-secret-value" {
t.Errorf("GatewaySecret = %q, want my-secret-value (whitespace trimmed)", cfg.GatewaySecret)
}
}

func TestMaxRequestBytesDefault(t *testing.T) {
cfg, _ := config.Load("/tmp/no-file-maxreq.toml")
if cfg.MaxRequestBytes <= 0 {
t.Error("MaxRequestBytes should have a positive default")
}
}

func TestOAuthAndChatTimeoutDefaults(t *testing.T) {
cfg, _ := config.Load("/tmp/no-file-timeouts.toml")
if cfg.ChatTimeoutSeconds <= 0 {
t.Error("ChatTimeoutSeconds should have a positive default")
}
if cfg.OAuthTimeoutSeconds <= 0 {
t.Error("OAuthTimeoutSeconds should have a positive default")
}
if cfg.OAuthDefaultExpirySeconds <= 0 {
t.Error("OAuthDefaultExpirySeconds should have a positive default")
}
}

func TestGatewaySecretFileEnvVar(t *testing.T) {
dir := t.TempDir()
secretFile := dir + "/sec.txt"
if err := os.WriteFile(secretFile, []byte("env-secret\n"), 0o600); err != nil {
t.Fatal(err)
}
t.Setenv("GATEWAY_SECRET_FILE", secretFile)
cfg, err := config.Load("/tmp/no-file-gs.toml")
if err != nil {
t.Fatal(err)
}
if cfg.GatewaySecret != "env-secret" {
t.Errorf("GatewaySecret = %q, want env-secret", cfg.GatewaySecret)
}
}

func TestGatewaySecretEnvOverridesFile(t *testing.T) {
dir := t.TempDir()
secretFile := dir + "/sec2.txt"
if err := os.WriteFile(secretFile, []byte("file-secret\n"), 0o600); err != nil {
t.Fatal(err)
}
t.Setenv("GATEWAY_SECRET_FILE", secretFile)
t.Setenv("GATEWAY_SECRET", "env-wins")
cfg, err := config.Load("/tmp/no-file-gs2.toml")
if err != nil {
t.Fatal(err)
}
if cfg.GatewaySecret != "env-wins" {
t.Errorf("GatewaySecret = %q, want env-wins (env GATEWAY_SECRET overrides file)", cfg.GatewaySecret)
}
}

func TestMaxRequestBytesEnvVar(t *testing.T) {
t.Setenv("MAX_REQUEST_BYTES", "2048")
cfg, err := config.Load("/tmp/no-file-mrb.toml")
if err != nil {
t.Fatal(err)
}
if cfg.MaxRequestBytes != 2048 {
t.Errorf("MaxRequestBytes = %d, want 2048", cfg.MaxRequestBytes)
}
}
