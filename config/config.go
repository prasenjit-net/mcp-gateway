package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr       string `yaml:"listen_addr"`
	DataDir          string `yaml:"data_dir"`
	LogLevel         string `yaml:"log_level"`
	MaxResponseBytes int64  `yaml:"max_response_bytes"`
	UIDevProxy       string `yaml:"ui_dev_proxy"`
	GatewaySecret    string `yaml:"-"`
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:       ":8080",
		DataDir:          "./data",
		LogLevel:         "info",
		MaxResponseBytes: 1048576,
	}

	if data, err := os.ReadFile("gateway.yaml"); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

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

	return cfg, nil
}
