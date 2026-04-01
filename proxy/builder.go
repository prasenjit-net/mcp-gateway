package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/prasenjit-net/mcp-gateway/auth"
	"github.com/prasenjit-net/mcp-gateway/spec"
)

// allowedMethods is the set of HTTP methods the proxy will forward.
var allowedMethods = map[string]struct{}{
	"GET": {}, "POST": {}, "PUT": {}, "PATCH": {},
	"DELETE": {}, "HEAD": {}, "OPTIONS": {},
}

type BuildInput struct {
	Tool        *spec.ToolDefinition
	Arguments   map[string]interface{}
	InboundAuth *auth.InboundAuth
	Configured  auth.Authenticator
}

func Build(ctx context.Context, input BuildInput) (*http.Request, error) {
	tool := input.Tool
	args := make(map[string]interface{})
	for k, v := range input.Arguments {
		args[k] = v
	}

	// Validate the upstream base URL before touching it.
	if _, err := url.ParseRequestURI(strings.TrimRight(tool.Upstream, "/")); err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	pathStr := tool.PathTemplate
	for k, v := range args {
		placeholder := "{" + k + "}"
		if strings.Contains(pathStr, placeholder) {
			// URL-encode the value to prevent path injection.
			pathStr = strings.ReplaceAll(pathStr, placeholder, url.PathEscape(fmt.Sprintf("%v", v)))
			delete(args, k)
		}
	}

	rawURL := strings.TrimRight(tool.Upstream, "/") + pathStr

	method := strings.ToUpper(tool.Method)
	if _, ok := allowedMethods[method]; !ok {
		return nil, fmt.Errorf("disallowed HTTP method: %s", tool.Method)
	}

	var req *http.Request
	var err error

	switch method {
	case "GET", "DELETE", "HEAD":
		req, err = http.NewRequestWithContext(ctx, method, rawURL, nil)
		if err != nil {
			return nil, err
		}
		if len(args) > 0 {
			q := req.URL.Query()
			for k, v := range args {
				q.Set(k, fmt.Sprintf("%v", v))
			}
			req.URL.RawQuery = q.Encode()
		}
	default:
		var bodyBytes []byte
		if len(args) > 0 {
			bodyBytes, err = json.Marshal(args)
			if err != nil {
				return nil, err
			}
		}
		var bodyReader *bytes.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		} else {
			bodyReader = bytes.NewReader([]byte{})
		}
		req, err = http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
		if err != nil {
			return nil, err
		}
		if len(args) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	req.Header.Set("X-MCP-Gateway", "1")

	if err := auth.ApplyChain(req, input.InboundAuth, input.Tool, input.Configured); err != nil {
		return nil, err
	}

	return req, nil
}
