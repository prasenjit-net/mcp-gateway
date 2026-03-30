package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prasenjit-net/mcp-gateway/auth"
	"github.com/prasenjit-net/mcp-gateway/buildinfo"
	"github.com/prasenjit-net/mcp-gateway/config"
	"github.com/prasenjit-net/mcp-gateway/proxy"
	"github.com/prasenjit-net/mcp-gateway/registry"
	"github.com/prasenjit-net/mcp-gateway/store"
	"github.com/prasenjit-net/mcp-gateway/telemetry"
)

type HandlerDeps struct {
	Registry       *registry.Registry
	Proxy          *proxy.Proxy
	Store          store.Store
	Config         *config.Config
	Authenticators map[string]auth.Authenticator
	AuthMu         sync.RWMutex
}

func (h *HandlerDeps) Handle(ctx context.Context, req *Request, inbound *auth.InboundAuth) *Response {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(req)
	case "initialized":
		return nil
	case "ping":
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	case "tools/list":
		return h.handleToolsList(req)
	case "tools/call":
		return h.handleToolsCall(ctx, req, inbound)
	case "resources/list":
		return h.handleResourcesList(req)
	case "resources/templates/list":
		return h.handleResourceTemplatesList(req)
	case "resources/read":
		return h.handleResourcesRead(ctx, req, inbound)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			},
		}
	}
}

func (h *HandlerDeps) handleInitialize(req *Request) *Response {
	result := InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{ListChanged: true},
			Resources: &ResourcesCapability{Subscribe: false, ListChanged: true},
		},
		ServerInfo: ServerInfo{
			Name:    "mcp-gateway",
			Version: buildinfo.Version,
		},
	}
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (h *HandlerDeps) handleToolsList(req *Request) *Response {
	tools := h.Registry.List()
	mcpTools := make([]Tool, 0, len(tools))
	for _, t := range tools {
		mcpTools = append(mcpTools, Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: ListToolsResult{Tools: mcpTools}}
}

func (h *HandlerDeps) handleToolsCall(ctx context.Context, req *Request, inbound *auth.InboundAuth) *Response {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "invalid params: " + err.Error()},
		}
	}

	tool, ok := h.Registry.Get(params.Name)
	if !ok {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "tool not found: " + params.Name},
		}
	}

	configured := h.getAuthenticator(tool.SpecID)

	start := time.Now()
	httpReq, err := proxy.Build(ctx, proxy.BuildInput{
		Tool:        tool,
		Arguments:   params.Arguments,
		InboundAuth: inbound,
		Configured:  configured,
	})
	if err != nil {
		_ = h.Store.IncrementStats(tool.OperationID, time.Since(start).Milliseconds(), true)
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  CallToolResult{Content: []proxy.MCPContent{{Type: "text", Text: "error building request: " + err.Error()}}, IsError: true},
		}
	}

	resp, err := h.Proxy.DoMTLS(httpReq, tool.MTLSEnabled)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		_ = h.Store.IncrementStats(tool.OperationID, latencyMs, true)
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  CallToolResult{Content: []proxy.MCPContent{{Type: "text", Text: "error calling upstream: " + err.Error()}}, IsError: true},
		}
	}

	content, err := proxy.MapResponse(resp, h.Config.MaxResponseBytes)
	isError := err != nil || resp.StatusCode >= 400
	_ = h.Store.IncrementStats(tool.OperationID, latencyMs, isError)

	// Prometheus metrics
	status := "success"
	if isError {
		status = "error"
	}
	telemetry.ToolCallsTotal.WithLabelValues(tool.SpecID, tool.OperationID, status).Inc()
	telemetry.ProxyDuration.WithLabelValues(tool.SpecID, tool.OperationID).Observe(float64(latencyMs) / 1000)

	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  CallToolResult{Content: []proxy.MCPContent{{Type: "text", Text: "error reading response: " + err.Error()}}, IsError: true},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  CallToolResult{Content: content, IsError: isError},
	}
}

func (h *HandlerDeps) getAuthenticator(specID string) auth.Authenticator {
	h.AuthMu.RLock()
	if a, ok := h.Authenticators[specID]; ok {
		h.AuthMu.RUnlock()
		return a
	}
	h.AuthMu.RUnlock()

	h.AuthMu.Lock()
	defer h.AuthMu.Unlock()

	if a, ok := h.Authenticators[specID]; ok {
		return a
	}

	authCfg, err := h.Store.GetAuth(specID)
	if err != nil {
		return nil
	}

	a, err := auth.NewAuthenticator(authCfg, h.Config.GatewaySecret)
	if err != nil {
		return nil
	}

	h.Authenticators[specID] = a
	return a
}

func (h *HandlerDeps) handleResourcesList(req *Request) *Response {
	resources := h.Registry.ListStaticResources()
	mcpResources := make([]Resource, 0, len(resources))
	for _, r := range resources {
		mcpResources = append(mcpResources, Resource{
			URI:         "gateway://resources/" + r.ID,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MimeType,
		})
	}
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: ListResourcesResult{Resources: mcpResources}}
}

func (h *HandlerDeps) handleResourceTemplatesList(req *Request) *Response {
	resources := h.Registry.ListTemplateResources()
	templates := make([]ResourceTemplate, 0, len(resources))
	for _, r := range resources {
		uriTemplate := r.URITemplate
		if uriTemplate == "" {
			uriTemplate = "gateway://resources/" + r.ID
		}
		templates = append(templates, ResourceTemplate{
			URITemplate: uriTemplate,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MimeType,
		})
	}
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: ListResourceTemplatesResult{ResourceTemplates: templates}}
}

func (h *HandlerDeps) handleResourcesRead(ctx context.Context, req *Request, inbound *auth.InboundAuth) *Response {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errResponse(req.ID, -32602, "invalid params: "+err.Error())
	}

	uri := params.URI
	prefix := "gateway://resources/"
	if !strings.HasPrefix(uri, prefix) {
		return errResponse(req.ID, -32602, "unsupported resource URI: "+uri)
	}
	id := strings.TrimPrefix(uri, prefix)
	if idx := strings.IndexByte(id, '?'); idx >= 0 {
		id = id[:idx]
	}

	record, ok := h.Registry.GetResourceByID(id)
	if !ok {
		return errResponse(req.ID, -32002, "resource not found: "+id)
	}

	var content ResourceContent
	content.URI = uri
	content.MimeType = record.MimeType

	switch record.Type {
	case "file", "text":
		filePath := filepath.Join(h.Config.DataDir, record.FilePath)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return errResponse(req.ID, -32603, "error reading resource: "+err.Error())
		}
		if isBinaryMime(record.MimeType) {
			content.Blob = base64.StdEncoding.EncodeToString(data)
		} else {
			content.Text = string(data)
		}
	case "upstream":
		httpReq, err := http.NewRequestWithContext(ctx, "GET", record.UpstreamURL, nil)
		if err != nil {
			return errResponse(req.ID, -32603, "error building upstream request: "+err.Error())
		}
		if record.PassthroughAuth && inbound != nil && inbound.Authorization != "" {
			httpReq.Header.Set("Authorization", inbound.Authorization)
		}
		if record.PassthroughCookies && inbound != nil && inbound.Cookie != "" {
			httpReq.Header.Set("Cookie", inbound.Cookie)
		}
		resp, err := h.Proxy.DoMTLS(httpReq, false)
		if err != nil {
			return errResponse(req.ID, -32603, "error fetching upstream: "+err.Error())
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, h.Config.MaxResponseBytes))
		if err != nil {
			return errResponse(req.ID, -32603, "error reading upstream response: "+err.Error())
		}
		content.Text = string(body)
		if ct := resp.Header.Get("Content-Type"); ct != "" && content.MimeType == "" {
			content.MimeType = ct
		}
	default:
		return errResponse(req.ID, -32603, "unknown resource type: "+record.Type)
	}

	return &Response{JSONRPC: "2.0", ID: req.ID, Result: ReadResourceResult{Contents: []ResourceContent{content}}}
}

func errResponse(id interface{}, code int, msg string) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg}}
}

func isBinaryMime(mime string) bool {
	return strings.HasPrefix(mime, "image/") ||
		strings.HasPrefix(mime, "audio/") ||
		strings.HasPrefix(mime, "video/") ||
		mime == "application/octet-stream" ||
		mime == "application/pdf"
}
