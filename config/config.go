package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// DefaultConfigFile is the default config file name used by init and serve.
const DefaultConfigFile = "config.toml"

type TLSConfig struct {
	Enabled  bool   `toml:"enabled"`
	CertFile string `toml:"cert_file"`
	KeyFile  string `toml:"key_file"`
}

type MTLSConfig struct {
	CertFile string `toml:"cert_file"`
	KeyFile  string `toml:"key_file"`
	CAFile   string `toml:"ca_file"`
}

// CORSConfig controls Cross-Origin Resource Sharing headers on all API routes.
// Leave AllowedOrigins empty (default) to deny all cross-origin requests.
// Use ["*"] to allow any origin (not recommended for production).
type CORSConfig struct {
	// AllowedOrigins is the list of origins permitted to make cross-origin requests.
	// An empty list (default) disables CORS — only same-origin requests are served.
	AllowedOrigins []string `toml:"allowed_origins"`
	// AllowedMethods defaults to the standard set when not specified.
	AllowedMethods []string `toml:"allowed_methods"`
	// AllowedHeaders defaults to Content-Type and Authorization when not specified.
	AllowedHeaders []string `toml:"allowed_headers"`
}

type Config struct {
	ListenAddr       string `toml:"listen_addr"`
	DataDir          string `toml:"data_dir"`
	LogLevel         string `toml:"log_level"`
	MaxResponseBytes int64  `toml:"max_response_bytes"`
	// MaxRequestBytes caps inbound JSON body size (default 1 MiB).
	MaxRequestBytes int64  `toml:"max_request_bytes"`
	UIDevProxy      string `toml:"ui_dev_proxy"`

	// GatewaySecret is used to encrypt stored credentials.
	// Prefer GatewaySecretFile; this field is only populated from env GATEWAY_SECRET.
	GatewaySecret string `toml:"-"`
	// GatewaySecretFile is the path to a file whose content is used as GatewaySecret.
	// Reading from a file avoids exposing the secret in /proc/<pid>/environ.
	GatewaySecretFile string `toml:"gateway_secret_file"`

	// AdminPassword protects the admin UI and API with form-based authentication.
	// Set via ADMIN_PASSWORD env var or admin.password in config file.
	// When empty, the admin interface is accessible without authentication (dev mode).
	AdminPassword string `toml:"admin_password"`
	// AdminSessionTTL is the lifetime of an admin session cookie in hours (default 24).
	AdminSessionTTL int `toml:"admin_session_ttl_hours"`

	// OpenAI settings for the built-in chat/test client.
	// The API key is never exposed to the browser.
	OpenAIAPIKey string `toml:"openai_api_key"`
	OpenAIModel  string `toml:"openai_model"`
	// ChatTimeoutSeconds is the HTTP timeout when proxying requests to OpenAI (default 60).
	ChatTimeoutSeconds int `toml:"chat_timeout_seconds"`

	// OAuthTimeoutSeconds is the HTTP timeout for OAuth2 token endpoint requests (default 10).
	OAuthTimeoutSeconds int `toml:"oauth_timeout_seconds"`
	// OAuthDefaultExpirySeconds is used when the token endpoint omits expires_in (default 300).
	OAuthDefaultExpirySeconds int `toml:"oauth_default_expiry_seconds"`

	TLS  TLSConfig  `toml:"tls"`
	MTLS MTLSConfig `toml:"mtls"`
	CORS CORSConfig `toml:"cors"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:                ":8080",
		DataDir:                   "./data",
		LogLevel:                  "info",
		MaxResponseBytes:          1048576,
		MaxRequestBytes:           1048576,
		OpenAIModel:               "gpt-4o",
		AdminSessionTTL:           24,
		ChatTimeoutSeconds:        60,
		OAuthTimeoutSeconds:       10,
		OAuthDefaultExpirySeconds: 300,
		CORS: CORSConfig{
			AllowedOrigins: []string{},
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
		},
	}
}

// Load reads configuration from the given TOML file path (if it exists),
// then applies environment variable overrides.
// path is typically the value of the --config flag (default: config.toml).
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		path = DefaultConfigFile
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// Environment variables override file values.
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("MAX_RESPONSE_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxResponseBytes = n
		}
	}
	if v := os.Getenv("MAX_REQUEST_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxRequestBytes = n
		}
	}
	if v := os.Getenv("UI_DEV_PROXY"); v != "" {
		cfg.UIDevProxy = v
	}

	// Gateway secret: prefer file over env to avoid /proc/<pid>/environ exposure.
	if v := os.Getenv("GATEWAY_SECRET_FILE"); v != "" {
		cfg.GatewaySecretFile = v
	}
	if cfg.GatewaySecretFile != "" {
		if raw, err := os.ReadFile(cfg.GatewaySecretFile); err == nil {
			cfg.GatewaySecret = strings.TrimSpace(string(raw))
		}
	}
	// Env GATEWAY_SECRET overrides file if explicitly set.
	if v := os.Getenv("GATEWAY_SECRET"); v != "" {
		cfg.GatewaySecret = v
	}

	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		cfg.OpenAIModel = v
	}
	if cfg.OpenAIModel == "" {
		cfg.OpenAIModel = "gpt-4o"
	}
	if cfg.AdminSessionTTL <= 0 {
		cfg.AdminSessionTTL = 24
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = 1048576
	}
	if cfg.ChatTimeoutSeconds <= 0 {
		cfg.ChatTimeoutSeconds = 60
	}
	if cfg.OAuthTimeoutSeconds <= 0 {
		cfg.OAuthTimeoutSeconds = 10
	}
	if cfg.OAuthDefaultExpirySeconds <= 0 {
		cfg.OAuthDefaultExpirySeconds = 300
	}
	// Apply CORS defaults if not set by file or env.
	if len(cfg.CORS.AllowedMethods) == 0 {
		cfg.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if len(cfg.CORS.AllowedHeaders) == 0 {
		cfg.CORS.AllowedHeaders = []string{"Content-Type", "Authorization"}
	}

	// Resolve default TLS paths relative to data_dir.
	if cfg.TLS.CertFile == "" {
		cfg.TLS.CertFile = filepath.Join(cfg.DataDir, "server.crt")
	}
	if cfg.TLS.KeyFile == "" {
		cfg.TLS.KeyFile = filepath.Join(cfg.DataDir, "server.key")
	}
	// mTLS defaults to the same cert/key as the server TLS cert.
	if cfg.MTLS.CertFile == "" {
		cfg.MTLS.CertFile = cfg.TLS.CertFile
	}
	if cfg.MTLS.KeyFile == "" {
		cfg.MTLS.KeyFile = cfg.TLS.KeyFile
	}
	if cfg.MTLS.CAFile == "" {
		cfg.MTLS.CAFile = cfg.TLS.CertFile
	}

	return cfg, nil
}

// DefaultConfigContent returns the template written by `mcp-gateway init`.
const DefaultConfigContent = `# MCP Gateway — configuration file
# Reference: https://github.com/prasenjit-net/mcp-gateway

# Address and port the server listens on.
listen_addr = ":8080"

# Directory where specs, auth credentials, and stats are persisted.
data_dir = "./data"

# Log level: debug | info | warn | error
log_level = "info"

# Maximum bytes read from an upstream API response (default 1 MiB).
max_response_bytes = 1048576

# Maximum bytes accepted in an inbound request body (default 1 MiB).
max_request_bytes = 1048576

# ── Gateway secret ────────────────────────────────────────────────────────────
# Used to encrypt stored credentials (OAuth2 client secrets, etc.).
# Prefer gateway_secret_file over env GATEWAY_SECRET to avoid exposing the
# secret in /proc/<pid>/environ on Linux.
# gateway_secret_file = "/run/secrets/gateway_secret"
# (Alternatively: export GATEWAY_SECRET="..." in the process environment)

# ── Admin authentication ───────────────────────────────────────────────────────
# Set a password to protect the admin UI and API with form-based authentication.
# Can also be set via the ADMIN_PASSWORD environment variable.
# When empty, the admin interface is accessible without authentication (dev mode only).
# admin_password = "change-me-in-production"

# Admin session cookie lifetime in hours (default 24).
admin_session_ttl_hours = 24

# ── OpenAI settings (built-in chat/test client) ───────────────────────────────
# Set the API key here OR via the OPENAI_API_KEY environment variable.
# The key is never sent to the browser.
# openai_api_key = "sk-..."
openai_model = "gpt-4o"

# HTTP timeout in seconds when proxying requests to OpenAI (default 60).
chat_timeout_seconds = 60

# ── OAuth2 token fetch settings ───────────────────────────────────────────────
# HTTP timeout for OAuth2 token endpoint requests (default 10 seconds).
oauth_timeout_seconds = 10
# Fallback token lifetime when expires_in is absent from the token response.
# Conservative default of 300 s prevents stale token use (default 300).
oauth_default_expiry_seconds = 300

# ── CORS (Cross-Origin Resource Sharing) ──────────────────────────────────────
# allowed_origins: list of origins permitted to make cross-origin requests.
# Leave empty (default) for same-origin only. Use ["*"] to allow all origins.
# Example: allowed_origins = ["https://app.example.com", "https://admin.example.com"]
[cors]
allowed_origins = []
# allowed_methods defaults to: GET, POST, PUT, PATCH, DELETE, OPTIONS
# allowed_headers defaults to: Content-Type, Authorization

# ── TLS (server) ──────────────────────────────────────────────────────────────
# When enabled, the server serves both HTTP and HTTPS on the same port via cmux.
# cert_file defaults to {data_dir}/server.crt
# key_file  defaults to {data_dir}/server.key
[tls]
enabled = false
# cert_file = "data/server.crt"
# key_file  = "data/server.key"

# ── mTLS (upstream client certificate) ───────────────────────────────────────
# Configure a client certificate to present to upstream servers that require mTLS.
# Enable mTLS per-spec in the admin UI.
# Defaults to the same cert/key as [tls] when not set.
[mtls]
# cert_file = "data/server.crt"
# key_file  = "data/server.key"
# ca_file   = "data/server.crt"
`
