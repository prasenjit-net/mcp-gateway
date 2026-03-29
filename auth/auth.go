package auth

import (
	"net/http"

	"github.com/prasenjit-net/mcp-gateway/spec"
	"github.com/prasenjit-net/mcp-gateway/store"
)

type Authenticator interface {
	Apply(req *http.Request) error
}

type InboundAuth struct {
	Authorization string
	Cookie        string
	ExtraHeaders  map[string]string
}

type noopAuthenticator struct{}

func (n *noopAuthenticator) Apply(req *http.Request) error { return nil }

func NewAuthenticator(cfg *store.AuthConfig, secret string) (Authenticator, error) {
	if cfg == nil || cfg.Type == "" || cfg.Type == "none" {
		return &noopAuthenticator{}, nil
	}
	switch cfg.Type {
	case "api-key":
		return newAPIKeyAuth(cfg.Config, secret)
	case "bearer":
		return newBearerAuth(cfg.Config, secret)
	case "basic":
		return newBasicAuth(cfg.Config, secret)
	case "oauth2":
		return newOAuth2Auth(cfg.Config, secret)
	default:
		return &noopAuthenticator{}, nil
	}
}

func ApplyChain(req *http.Request, inbound *InboundAuth, tool *spec.ToolDefinition, configured Authenticator) error {
	if inbound != nil {
		if tool.PassthroughAuth && inbound.Authorization != "" {
			req.Header.Set("Authorization", inbound.Authorization)
		}
		if tool.PassthroughCookies && inbound.Cookie != "" {
			req.Header.Set("Cookie", inbound.Cookie)
		}
		for _, h := range tool.PassthroughHeaders {
			if v, ok := inbound.ExtraHeaders[h]; ok && v != "" {
				req.Header.Set(h, v)
			}
		}
	}

	if configured != nil {
		return configured.Apply(req)
	}
	return nil
}
