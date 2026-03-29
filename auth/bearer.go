package auth

import (
	"encoding/json"
	"net/http"
)

type bearerConfig struct {
	Token string `json:"token"`
}

type bearerAuth struct {
	token string
}

func newBearerAuth(raw json.RawMessage, secret string) (Authenticator, error) {
	var cfg bearerConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	token := cfg.Token
	if secret != "" && token != "" {
		if dec, err := Decrypt(secret, []byte(token)); err == nil {
			token = string(dec)
		}
	}
	return &bearerAuth{token: token}, nil
}

func (b *bearerAuth) Apply(req *http.Request) error {
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
	return nil
}
