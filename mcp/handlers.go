package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/prasenjit-net/mcp-gateway/auth"
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
			Tools: &ToolsCapability{ListChanged: true},
		},
		ServerInfo: ServerInfo{
			Name:    "mcp-gateway",
			Version: "0.1.0",
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
