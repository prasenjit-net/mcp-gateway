package store

import (
	"encoding/json"
	"time"
)

type SpecRecord struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	UpstreamURL        string    `json:"upstream_url"`
	SpecRaw            string    `json:"spec_raw"`
	PassthroughAuth    bool      `json:"passthrough_auth"`
	PassthroughCookies bool      `json:"passthrough_cookies"`
	PassthroughHeaders []string  `json:"passthrough_headers"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type OperationRecord struct {
	ID          string   `json:"id"`
	SpecID      string   `json:"spec_id"`
	OperationID string   `json:"operation_id"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Enabled     bool     `json:"enabled"`
}

type ToolStats struct {
	OperationID    string    `json:"operation_id"`
	CallCount      int64     `json:"call_count"`
	ErrorCount     int64     `json:"error_count"`
	TotalLatencyMs int64     `json:"total_latency_ms"`
	LastCalledAt   time.Time `json:"last_called_at"`
}

type AuthConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}
