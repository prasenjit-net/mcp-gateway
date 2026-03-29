package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
)

type basicConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type basicAuth struct {
	encoded string
}

func newBasicAuth(raw json.RawMessage, secret string) (Authenticator, error) {
	var cfg basicConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if secret != "" {
		if dec, err := Decrypt(secret, []byte(cfg.Password)); err == nil {
			cfg.Password = string(dec)
		}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(cfg.Username + ":" + cfg.Password))
	return &basicAuth{encoded: encoded}, nil
}

func (b *basicAuth) Apply(req *http.Request) error {
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Basic "+b.encoded)
	}
	return nil
}
