package proxy_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/prasenjit-net/mcp-gateway/proxy"
	"github.com/prasenjit-net/mcp-gateway/spec"
)

func makeTool(method, upstream, path string) *spec.ToolDefinition {
	return &spec.ToolDefinition{
		Method:       method,
		Upstream:     upstream,
		PathTemplate: path,
	}
}

// ── Build — path parameters ───────────────────────────────────────────────────

func TestBuildPathParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	tool := makeTool("GET", srv.URL, "/users/{id}")
	input := proxy.BuildInput{
		Tool:      tool,
		Arguments: map[string]interface{}{"id": "42"},
	}
	req, err := proxy.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.HasSuffix(req.URL.Path, "/users/42") {
		t.Errorf("path = %q, want /users/42", req.URL.Path)
	}
}

func TestBuildQueryParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	tool := makeTool("GET", srv.URL, "/search")
	input := proxy.BuildInput{
		Tool:      tool,
		Arguments: map[string]interface{}{"q": "hello", "limit": 10},
	}
	req, err := proxy.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	vals, _ := url.ParseQuery(req.URL.RawQuery)
	if vals.Get("q") != "hello" {
		t.Errorf("query param q = %q, want hello", vals.Get("q"))
	}
	if vals.Get("limit") != "10" {
		t.Errorf("query param limit = %q, want 10", vals.Get("limit"))
	}
}

func TestBuildPOSTBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	tool := makeTool("POST", srv.URL, "/items")
	input := proxy.BuildInput{
		Tool:      tool,
		Arguments: map[string]interface{}{"name": "widget", "count": 5},
	}
	req, err := proxy.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", req.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(req.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if m["name"] != "widget" {
		t.Errorf("body.name = %v", m["name"])
	}
}

func TestBuildMCPGatewayHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	tool := makeTool("GET", srv.URL, "/ping")
	input := proxy.BuildInput{Tool: tool, Arguments: map[string]interface{}{}}
	req, err := proxy.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if req.Header.Get("X-MCP-Gateway") != "1" {
		t.Errorf("X-MCP-Gateway = %q, want 1", req.Header.Get("X-MCP-Gateway"))
	}
}

func TestBuildDELETE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	tool := makeTool("DELETE", srv.URL, "/items/{id}")
	input := proxy.BuildInput{
		Tool:      tool,
		Arguments: map[string]interface{}{"id": "99"},
	}
	req, err := proxy.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if req.Method != "DELETE" {
		t.Errorf("Method = %q, want DELETE", req.Method)
	}
}

func TestBuildPATCHBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	tool := makeTool("PATCH", srv.URL, "/items/{id}")
	input := proxy.BuildInput{
		Tool:      tool,
		Arguments: map[string]interface{}{"id": "1", "status": "done"},
	}
	req, err := proxy.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// id is consumed by path; status should be in body
	body, _ := io.ReadAll(req.Body)
	var m map[string]interface{}
	json.Unmarshal(body, &m) //nolint:errcheck
	if m["status"] != "done" {
		t.Errorf("body.status = %v", m["status"])
	}
	if _, hasID := m["id"]; hasID {
		t.Error("path param 'id' should not appear in body")
	}
}

// ── MapResponse ───────────────────────────────────────────────────────────────

func makeResp(status int, ct, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{ct}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestMapResponseJSON(t *testing.T) {
	resp := makeResp(200, "application/json", `{"key":"value"}`)
	contents, err := proxy.MapResponse(resp, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 || contents[0].Type != "text" {
		t.Errorf("expected one text content, got %+v", contents)
	}
	if !strings.Contains(contents[0].Text, `"key"`) {
		t.Errorf("text does not contain key, got: %s", contents[0].Text)
	}
}

func TestMapResponseText(t *testing.T) {
	resp := makeResp(200, "text/plain", "hello world")
	contents, err := proxy.MapResponse(resp, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 || contents[0].Text != "hello world" {
		t.Errorf("unexpected content: %+v", contents)
	}
}

func TestMapResponseBinary(t *testing.T) {
	resp := makeResp(200, "image/png", "\x89PNG\r\n\x1a\n")
	contents, err := proxy.MapResponse(resp, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 || contents[0].Type != "resource" {
		t.Errorf("expected resource content, got %+v", contents)
	}
	if contents[0].Data == "" {
		t.Error("Data should not be empty for binary content")
	}
}

func TestMapResponseTruncated(t *testing.T) {
	resp := makeResp(200, "text/plain", "abcdefghij")
	contents, err := proxy.MapResponse(resp, 5) // limit to 5 bytes
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contents[0].Text, "[truncated]") {
		t.Errorf("expected truncation notice, got: %s", contents[0].Text)
	}
}

func TestMapResponseEmptyContentType(t *testing.T) {
	resp := makeResp(200, "", "raw bytes")
	contents, err := proxy.MapResponse(resp, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 || contents[0].Type != "resource" {
		t.Errorf("expected resource for empty content-type, got %+v", contents)
	}
	if contents[0].MimeType == "" {
		t.Error("MimeType should be set to fallback for empty CT")
	}
}

func TestMapResponseHTMLText(t *testing.T) {
	resp := makeResp(200, "text/html; charset=utf-8", "<h1>Hello</h1>")
	contents, err := proxy.MapResponse(resp, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if contents[0].Type != "text" {
		t.Errorf("text/* should map to text type, got %q", contents[0].Type)
	}
}

func TestBuildDisallowedMethod(t *testing.T) {
tool := &spec.ToolDefinition{
Name:         "test",
Upstream:     "http://example.com",
PathTemplate: "/path",
Method:       "TRACE",
}
_, err := proxy.Build(context.Background(), proxy.BuildInput{Tool: tool})
if err == nil {
t.Error("expected error for disallowed HTTP method TRACE")
}
}

func TestBuildPathEscaping(t *testing.T) {
	tool := &spec.ToolDefinition{
		Name:         "test",
		Upstream:     "http://example.com",
		PathTemplate: "/items/{id}",
		Method:       "GET",
	}
	req, err := proxy.Build(context.Background(), proxy.BuildInput{
		Tool:      tool,
		Arguments: map[string]interface{}{"id": "../../etc/passwd"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The slash in the traversal attempt must be percent-encoded in RawPath,
	// so the upstream receives it as a single opaque path segment, not a traversal.
	rawPath := req.URL.RawPath
	if rawPath == "" {
		rawPath = req.URL.Path // fallback if no encoding needed
	}
	if strings.Contains(rawPath, "../") {
		t.Errorf("raw path contains unencoded traversal: %s", rawPath)
	}
}

func TestBuildInvalidUpstreamURL(t *testing.T) {
tool := &spec.ToolDefinition{
Name:         "test",
Upstream:     "not-a-url",
PathTemplate: "/path",
Method:       "GET",
}
_, err := proxy.Build(context.Background(), proxy.BuildInput{Tool: tool})
if err == nil {
t.Error("expected error for invalid upstream URL")
}
}
