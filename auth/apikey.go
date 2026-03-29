package auth

import (
	"encoding/json"
	"net/http"
)

type apiKeyConfig struct {
	Header string `json:"header"`
	Query  string `json:"query"`
	Value  string `json:"value"`
}

type apiKeyAuth struct {
	cfg apiKeyConfig
}

func newAPIKeyAuth(raw json.RawMessage, secret string) (Authenticator, error) {
	var cfg apiKeyConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if secret != "" && cfg.Value != "" {
		if dec, err := Decrypt(secret, []byte(cfg.Value)); err == nil {
			cfg.Value = string(dec)
		}
	}
	return &apiKeyAuth{cfg: cfg}, nil
}

func (a *apiKeyAuth) Apply(req *http.Request) error {
	if a.cfg.Header != "" {
		if req.Header.Get(a.cfg.Header) == "" {
			req.Header.Set(a.cfg.Header, a.cfg.Value)
		}
	}
	if a.cfg.Query != "" {
		q := req.URL.Query()
		if q.Get(a.cfg.Query) == "" {
			q.Set(a.cfg.Query, a.cfg.Value)
		}
		req.URL.RawQuery = q.Encode()
	}
	return nil
}
