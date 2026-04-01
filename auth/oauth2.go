package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type oauth2Config struct {
	TokenURL     string   `json:"token_url"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Scopes       []string `json:"scopes"`
	// TimeoutSeconds is the HTTP timeout for token requests (default 10).
	TimeoutSeconds int `json:"timeout_seconds"`
	// DefaultExpirySeconds is used when expires_in is absent (default 300).
	DefaultExpirySeconds int `json:"default_expiry_seconds"`
}

type oauth2Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

type oauth2Auth struct {
	cfg    oauth2Config
	mu     sync.Mutex
	cached *oauth2Token
}

func newOAuth2Auth(raw json.RawMessage, secret string) (Authenticator, error) {
	var cfg oauth2Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if secret != "" && cfg.ClientSecret != "" {
		if dec, err := Decrypt(secret, []byte(cfg.ClientSecret)); err == nil {
			cfg.ClientSecret = string(dec)
		}
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 10
	}
	if cfg.DefaultExpirySeconds <= 0 {
		cfg.DefaultExpirySeconds = 300
	}
	return &oauth2Auth{cfg: cfg}, nil
}

func (o *oauth2Auth) Apply(req *http.Request) error {
	token, err := o.getToken()
	if err != nil {
		return err
	}
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func (o *oauth2Auth) getToken() (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cached != nil && time.Until(o.cached.ExpiresAt) > 30*time.Second {
		return o.cached.AccessToken, nil
	}

	return o.fetchToken()
}

func (o *oauth2Auth) fetchToken() (string, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", o.cfg.ClientID)
	data.Set("client_secret", o.cfg.ClientSecret)
	if len(o.cfg.Scopes) > 0 {
		data.Set("scope", strings.Join(o.cfg.Scopes, " "))
	}

	client := &http.Client{Timeout: time.Duration(o.cfg.TimeoutSeconds) * time.Second}
	resp, err := client.PostForm(o.cfg.TokenURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	expiresIn := result.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = o.cfg.DefaultExpirySeconds
	}

	o.cached = &oauth2Token{
		AccessToken: result.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second),
	}

	return result.AccessToken, nil
}
