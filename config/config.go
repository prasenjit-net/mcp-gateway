package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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

type Config struct {
	ListenAddr       string `toml:"listen_addr"`
	DataDir          string `toml:"data_dir"`
	LogLevel         string `toml:"log_level"`
	MaxResponseBytes int64  `toml:"max_response_bytes"`
	UIDevProxy       string `toml:"ui_dev_proxy"`
	GatewaySecret    string `toml:"-"`

	// OpenAI settings for the built-in chat/test client.
	// The API key is never exposed to the browser.
	OpenAIAPIKey string `toml:"openai_api_key"`
	OpenAIModel  string `toml:"openai_model"`

	TLS  TLSConfig  `toml:"tls"`
	MTLS MTLSConfig `toml:"mtls"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:       ":8080",
		DataDir:          "./data",
		LogLevel:         "info",
		MaxResponseBytes: 1048576,
		OpenAIModel:      "gpt-4o",
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
	if v := os.Getenv("UI_DEV_PROXY"); v != "" {
		cfg.UIDevProxy = v
	}
	cfg.GatewaySecret = os.Getenv("GATEWAY_SECRET")
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		cfg.OpenAIModel = v
	}
	if cfg.OpenAIModel == "" {
		cfg.OpenAIModel = "gpt-4o"
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

# ── OpenAI settings (built-in chat/test client) ───────────────────────────
# Set the API key here OR via the OPENAI_API_KEY environment variable.
# The key is never sent to the browser.
# openai_api_key = "sk-..."
openai_model = "gpt-4o"

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
