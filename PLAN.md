# MCP Gateway — Project Plan

## Problem Statement

Build an **MCP Gateway** in Go — a single HTTP server that:
1. Lets administrators upload OpenAPI 3 specs via a rich React UI
2. Auto-generates MCP tools from those specs on the fly
3. Exposes all tools via **SSE and HTTP** MCP transports for AI agents
4. Proxies every tool call transparently to the real backend REST API
5. Includes a built-in **MCP chat client** in the Admin UI for interactive testing

No stdio transport. No database — all state stored as JSON files in a configurable
`data/` directory. Specs and configuration managed entirely through the Admin UI.

---

## High-Level Architecture

```
                        ┌──────────────────────────────────────────────────┐
                        │                  MCP Gateway                     │
                        │                                                  │
  Browser ──────────▶  │  /_ui/*          React SPA (embedded)            │
                        │   └─ Chat page   Built-in MCP test client        │
                        │                                                  │
  Admin ──────────────▶ │  /_api/*         Admin REST API                  │
                        │   ├─ POST /specs        upload OAI3 spec         │
                        │   ├─ GET  /specs        list registrations       │
                        │   ├─ GET  /specs/:id    spec detail + ops        │
                        │   ├─ PATCH/DELETE /specs/:id                    │
                        │   ├─ PATCH /specs/:id/operations/:opId           │
                        │   ├─ GET  /stats        dashboard metrics        │
                        │   └─ GET  /health                                │
                        │                                                  │
                        │  ┌──────────────┐   ┌─────────────────────────┐ │
                        │  │  JSON Store  │   │    Tool Registry        │ │
                        │  │ (data/ dir)  │──▶│  (in-memory, rebuilt    │ │
                        │  │             │   │   on spec change)        │ │
                        │  └──────────────┘   └──────────┬──────────────┘ │
                        │                               │                 │
  AI Agent ───────────▶ │  /mcp/sse        MCP SSE transport              │ │
                        │  /mcp/http       MCP HTTP (stateless POST)       │ │
                        │                  ┌────────────▼──────────────┐  │
                        │                  │  MCP Protocol Handler     │  │
                        │                  │  tools/list · tools/call  │  │
                        │                  └────────────┬──────────────┘  │
                        └───────────────────────────────┼─────────────────┘
                                                        │ HTTP proxy
                                                        ▼
                                             Backend REST API(s)
```

---

## URL Routing Map

| Path | Handler | Description |
|------|---------|-------------|
| `GET /_ui/*` | Static file server | Serves embedded React SPA |
| `GET /_ui/` | → `/_ui/index.html` | SPA entry point |
| `POST /_api/specs` | Admin API | Upload new OAI3 spec + upstream URL |
| `GET /_api/specs` | Admin API | List all registered specs |
| `GET /_api/specs/:id` | Admin API | Spec detail with parsed operation list |
| `PATCH /_api/specs/:id` | Admin API | Update upstream URL / auth / name |
| `DELETE /_api/specs/:id` | Admin API | Remove spec and all its tools |
| `GET /_api/specs/:id/operations` | Admin API | List operations with enabled flag |
| `PATCH /_api/specs/:id/operations/:opId` | Admin API | Enable / disable an operation |
| `GET /_api/stats` | Admin API | Global dashboard stats |
| `GET /_api/stats/tools` | Admin API | Per-tool call / error / latency stats |
| `GET /_api/health` | Admin API | Liveness probe |
| `GET /mcp/sse` | MCP SSE transport | Agent opens SSE stream here |
| `POST /mcp/sse/message` | MCP SSE transport | Agent posts JSON-RPC messages for SSE session |
| `POST /mcp/http` | MCP HTTP transport | Stateless single-request JSON-RPC endpoint |
| `GET /metrics` | Telemetry | Prometheus scrape endpoint |

---

## Components

### 1. Configuration (`config/`)
Minimal startup config (env vars + optional `gateway.yaml`):
- `listen_addr` — default `:8080`
- `data_dir` — path to JSON data directory, default `./data`
- `log_level` — `debug` | `info` | `warn` | `error`
- `max_response_bytes` — upstream response truncation limit
- `ui_dev_proxy` — optional URL to proxy `/_ui` to a Vite dev server (dev mode)

No specs or auth in startup config — all managed via the Admin UI.

### 2. JSON File Store (`store/`)
All persistent state stored as JSON files under `data_dir`. No database.

#### File layout
```
data/
├── specs/
│   ├── {spec-uuid}.json          # spec metadata + raw spec content
│   └── ...
├── operations/
│   ├── {spec-uuid}.json          # array of operations for a spec
│   └── ...
├── auth/
│   ├── {spec-uuid}.json          # AES-GCM encrypted auth config blob
│   └── ...
└── stats/
    └── tool_stats.json           # map of operationId → call stats
```

#### Go structs

```go
// data/specs/{id}.json
type SpecRecord struct {
    ID                 string    `json:"id"`
    Name               string    `json:"name"`
    UpstreamURL        string    `json:"upstream_url"`
    SpecRaw            string    `json:"spec_raw"`   // original YAML/JSON text
    PassthroughAuth    bool      `json:"passthrough_auth"`    // forward inbound Authorization header
    PassthroughCookies bool      `json:"passthrough_cookies"` // forward inbound Cookie header
    PassthroughHeaders []string  `json:"passthrough_headers"` // additional headers to forward
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}

// data/operations/{spec-id}.json
type OperationRecord struct {
    ID          string   `json:"id"`
    SpecID      string   `json:"spec_id"`
    OperationID string   `json:"operation_id"`
    Method      string   `json:"method"`
    Path        string   `json:"path"`
    Summary     string   `json:"summary"`
    Description string   `json:"description"`
    Tags        []string `json:"tags"`
    Enabled     bool     `json:"enabled"`
}

// data/stats/tool_stats.json
type ToolStats struct {
    OperationID    string    `json:"operation_id"`
    CallCount      int64     `json:"call_count"`
    ErrorCount     int64     `json:"error_count"`
    TotalLatencyMs int64     `json:"total_latency_ms"`
    LastCalledAt   time.Time `json:"last_called_at"`
}
```

- Store interface with methods: `SaveSpec`, `GetSpec`, `ListSpecs`, `DeleteSpec`, `SaveOperations`, `GetOperations`, `UpdateOperation`, `IncrementStats`, `GetAllStats`
- File writes are atomic (write to `.tmp` then rename)
- Stats updates use an in-memory accumulator flushed to disk periodically (every 5s) to avoid write contention

### 3. OpenAPI Spec Parser (`spec/`)
Invoked when a spec is uploaded via Admin API:
- Accept YAML or JSON
- Resolve all `$ref` references (inline + bundled; no remote fetch from uploaded content)
- Extract per operation:
  - `operationId` → tool name (fallback: `{METHOD}_{sanitized_path}`)
  - `summary` + `description` → tool description
  - `tags` → grouping metadata
  - `parameters` (path, query, header) → JSON Schema properties
  - `requestBody` → merged into tool input schema
  - `responses` → documented for response mapping hints
- Validation: reject specs with duplicate operationIds within the same upload

### 4. Tool Registry (`registry/`)
In-memory index, rebuilt from the JSON store whenever:
- A new spec is uploaded
- An operation is enabled/disabled
- A spec is deleted

```go
type ToolDefinition struct {
    Name               string
    Description        string
    InputSchema        JSONSchema
    OperationID        string
    SpecID             string
    Method             string
    PathTemplate       string     // e.g. /pets/{petId}
    Upstream           string
    PassthroughAuth    bool
    PassthroughCookies bool
    PassthroughHeaders []string
}
```

- Thread-safe (RWMutex)
- On rebuild: sends `notifications/tools/list_changed` to all connected SSE clients

### 5. MCP Transports (`mcp/`)
Implements [MCP specification](https://modelcontextprotocol.io) over two transports:

#### SSE Transport (`GET /mcp/sse` + `POST /mcp/sse/message`)
1. Agent opens `GET /mcp/sse` — server sends `endpoint` event with message URL including `?sessionId=<uuid>`
2. Agent posts JSON-RPC messages to `POST /mcp/sse/message?sessionId=<id>`
3. Server pushes responses back over the open SSE stream for that session
4. Heartbeat every 15s keeps connection alive through proxies/load balancers
5. On disconnect: session cleaned up, goroutine cancelled

#### HTTP Transport (`POST /mcp/http`)
- Stateless: each request is a complete JSON-RPC call, response returned in HTTP body
- No session state; suitable for serverless / simple integrations
- Supports batched JSON-RPC arrays
- Same protocol handlers as SSE (initialize, tools/list, tools/call, ping)

#### Session management (SSE)
- Sessions stored in `sync.Map { sessionID → chan MCPMessage }`
- Max concurrent sessions configurable (default 100)

### 6. Request Builder (`proxy/builder.go`)
Converts a `tools/call` invocation into `*http.Request`:
1. Resolve path template → substitute path parameters from tool arguments
2. Append remaining arguments as query params (for GET/DELETE) or JSON body (for POST/PUT/PATCH)
3. Set `Content-Type: application/json` for body requests
4. **Auth resolution** — applied in priority order (first wins):
   - **Passthrough**: if the originating MCP request carried an `Authorization` header (SSE connection header or HTTP request header), forward it as-is to the upstream
   - **Configured auth**: if no passthrough auth, apply the per-spec configured auth strategy (api-key / bearer / basic / oauth2)
   - **None**: no auth header added
5. Forward `X-Request-ID`, `Accept-Language` and other safe headers from the MCP request
6. Set `X-MCP-Gateway: true`

> **Auth passthrough design**: When an agent connects to `/mcp/sse` or posts to `/mcp/http`,
> the gateway captures any `Authorization` header (and optionally `Cookie`) from that inbound
> request and attaches it to the session context. Every subsequent `tools/call` in that session
> uses the captured credentials when calling the upstream, allowing the AI agent to act on behalf
> of a real authenticated user without the gateway needing to manage those credentials.

### 7. HTTP Proxy (`proxy/proxy.go`)
- Connection-pooled `http.Client` per upstream (keyed by base URL)
- Configurable per-request timeout (default 30s)
- Maps HTTP status → MCP result:
  - `2xx` → success content
  - `4xx` / `5xx` → `isError: true` with response body as message
- Records latency + error counts into JSON stats store

### 8. Response Mapper (`proxy/mapper.go`)
- `application/json` → pretty-printed JSON string as MCP text content
- `text/*` → pass-through string
- Binary → base64-encoded string with MIME annotation
- Truncates at `max_response_bytes` with a `[truncated]` suffix

### 9. Authentication Module (`auth/`)

#### Per-spec configured auth
Pluggable strategies set via Admin UI, stored encrypted in `data/auth/{spec-id}.json`:

| Type | Behaviour |
|------|-----------|
| `none` | No modification (rely on passthrough) |
| `api-key` | Inject static key as header (`X-Api-Key`) or query param |
| `bearer` | `Authorization: Bearer <token>` |
| `basic` | `Authorization: Basic <b64(user:pass)>` |
| `oauth2-client-credentials` | POST to token URL, cache access token, auto-refresh on expiry |

Auth values set via Admin UI — stored AES-GCM encrypted in `data/auth/{spec-id}.json`.
Encryption key derived from `GATEWAY_SECRET` env var (required at startup).

#### Inbound auth passthrough (always applied, takes priority)
Any authentication presented **to** the MCP gateway is forwarded **to** the upstream REST API:

- `Authorization` header (Bearer token, Basic, API key as Bearer, etc.)
- `Cookie` header (optional — configurable per spec: `passthrough_cookies: true`)
- Custom headers listed in spec config `passthrough_headers: ["X-User-Id", "X-Tenant"]`

**How it works per transport:**
- **SSE**: `Authorization` captured from the `GET /mcp/sse` HTTP request → stored in session context → attached to every upstream request in that session
- **HTTP**: `Authorization` captured from each `POST /mcp/http` request → attached to that single upstream request

**Priority**: Inbound passthrough auth **overrides** configured auth for the same header.
If passthrough sends `Authorization`, configured `bearer` auth is skipped for that request.

### 10. Admin REST API (`admin/`)
All under `/_api/`:

#### Spec management
```
POST   /_api/specs              Upload spec (multipart: file + upstream_url + name + auth config)
GET    /_api/specs              List ALL specs { id, name, upstream, opCount, enabledCount }
GET    /_api/specs/:id          Spec detail + full operation list
PATCH  /_api/specs/:id          Update name / upstream_url / auth / passthrough settings
DELETE /_api/specs/:id          Delete spec → removes its tools from registry
```

> Multiple specs are supported simultaneously. Each spec is independent —
> its tools are all registered in the shared registry and exposed through the
> same MCP endpoint. Tool names are globally unique (spec name used as prefix
> if a collision is detected).

#### Operation management
```
GET    /_api/specs/:id/operations           List operations with enabled flag
PATCH  /_api/specs/:id/operations/:opId    { "enabled": true/false }
```

#### Stats & health
```
GET  /_api/stats         { totalSpecs, totalTools, enabledTools, totalCalls, errorRate, activeSessions }
GET  /_api/stats/tools   [ { operationId, name, callCount, errorCount, avgLatencyMs, lastCalledAt } ]
GET  /_api/health        { status: "ok", uptime, version }
```

### 11. Admin UI (`ui/`)
React + TypeScript, built with Vite, served from `/_ui` via Go's `embed.FS`.

#### Pages & Features

**Dashboard** (`/_ui/`)
- Summary cards: total specs, total tools, enabled tools, total calls today, error rate
- Recent activity feed (last 20 tool calls with status + latency)
- Active SSE sessions count

**Specs List** (`/_ui/specs`)
- Table of ALL registered specs: name, upstream URL, # operations, # enabled, uploaded date
- **"Upload New Spec" button** — opens upload drawer (can add as many specs as needed)
- Per-row actions: View, Edit upstream/auth/passthrough, Delete
- Each spec is independent; all their tools are merged into the single MCP endpoint

**Upload Drawer**
- Drag-and-drop or file picker (`.yaml`, `.yml`, `.json`)
- Fields: Display name, Upstream base URL
- Auth type selector + credential fields (masked inputs)
- Passthrough settings: enable auth passthrough toggle, custom passthrough headers list
- On submit: `POST /_api/specs`, shows parse results (# tools found, any name collisions auto-prefixed)

**Spec Detail** (`/_ui/specs/:id`)
- Spec metadata at top (name, upstream, auth type)
- Searchable/filterable operations table:
  - Columns: Method badge, Path, OperationId, Summary, Tags, Enable toggle
  - Bulk enable/disable
- "Copy MCP SSE URL" and "Copy MCP HTTP URL" buttons

**Stats** (`/_ui/stats`)
- Per-tool metrics table: calls, errors, avg latency, last called
- Sortable columns, search by tool name
- Latency bar chart per tool via Recharts

**Chat / MCP Test Client** (`/_ui/chat`)
- A conversational UI allowing interactive testing of the gateway's MCP tools using an OpenAI-compatible LLM
- Settings panel (persisted in `localStorage`):
  - OpenAI API key (masked input)
  - Model selector (e.g. `gpt-4o`, `gpt-4-turbo`, `gpt-3.5-turbo`)
  - MCP transport selector: SSE or HTTP
  - System prompt (optional)
- Chat interface:
  - Message thread with user / assistant bubbles
  - Tool call events shown inline: tool name, arguments (collapsible JSON), result (collapsible JSON)
  - "New conversation" button to clear history
- Under the hood:
  - Connects to `/mcp/sse` or `/mcp/http` using the selected transport
  - Sends `tools/list` on start, passes tool schemas to OpenAI `functions`/`tools`
  - On assistant tool_call: forwards to MCP `tools/call`, injects result back into conversation
  - Streams assistant responses using OpenAI streaming API (`stream: true`)
- **All API calls to OpenAI are made from the browser** — the gateway backend is not involved in LLM calls

#### Tech Stack
- React 18 + TypeScript
- Vite (build tool)
- TanStack Query (data fetching + cache)
- TanStack Router (client-side routing)
- shadcn/ui + Tailwind CSS (component library)
- Recharts (stats charts)
- OpenAI JS SDK (browser build, used in Chat page only)

### 12. Observability (`telemetry/`)
- Structured logging via `log/slog` (JSON in production, text in dev)
- Prometheus metrics at `/metrics`:
  - `mcp_tool_calls_total{spec, operation, status}`
  - `mcp_proxy_duration_seconds{spec, operation}`
  - `mcp_active_sessions`
  - `mcp_registry_tools_total{spec}`

---

## Data Flow: Upload Spec → Tool Available

```
Admin uploads petstore.yaml via UI
  │
  ▼
POST /_api/specs  { file, upstream: "https://petstore3.swagger.io/api/v3", name: "PetStore" }
  │
  ▼
Spec Parser: parse + resolve $refs → extract N operations
  │
  ├─▶ INSERT INTO specs (...)
  ├─▶ INSERT INTO operations (...) × N   (all enabled by default)
  └─▶ Tool Registry: rebuild from DB → N new ToolDefinitions
        │
        └─▶ Broadcast notifications/tools/list_changed to all SSE sessions
```

## Data Flow: Upload → Multiple Specs → Single MCP Endpoint

```
Admin uploads petstore.yaml          Admin uploads github.yaml
        │                                      │
        ▼                                      ▼
POST /_api/specs                      POST /_api/specs
  name: "petstore"                      name: "github"
  upstream: "https://petstore..."       upstream: "https://api.github.com"
        │                                      │
        ├── parse spec → 20 ops               ├── parse spec → 150 ops
        ├── store to data/specs/              ├── store to data/specs/
        └── rebuild registry ─────────────────┘
                  │
                  ▼
        Registry: 170 tools total
          petstore_getPetById, petstore_addPet ...
          github_listRepos, github_getIssue ...
                  │
                  ▼
        All tools served via /mcp/sse and /mcp/http
```

## Data Flow: `tools/call` with Auth Passthrough

```
Agent connects:  GET /mcp/sse
                 Authorization: Bearer <agent-token>
                       │
                       ▼
               SSE Session created
               session.InboundAuth = "Bearer <agent-token>"

Agent sends:  tools/call { name: "getPetById", arguments: { petId: 42 } }
                       │
                       ▼
Tool Registry: "getPetById" → ToolDefinition{
    GET /pets/{petId}, upstream: "https://petstore...",
    PassthroughAuth: true
}
                       │
                       ▼
Request Builder:
    URL:    https://petstore3.swagger.io/api/v3/pets/42
    Method: GET
    Auth priority:
      1. PassthroughAuth=true → use session.InboundAuth → "Bearer <agent-token>"
         (configured spec auth is skipped for Authorization header)
      2. Fallback → configured spec Bearer/ApiKey/OAuth2 token
                       │
                       ▼
HTTP Proxy → upstream server
  Headers: Authorization: Bearer <agent-token>
                       │
                       ▼
Response Mapper: 200 OK application/json → pretty JSON string
                       │
  ├─▶ Increment in-memory stats accumulator (flushed to JSON every 5s)
  └─▶ MCP Response: { content: [{ type: "text", text: "{...}" }] }
```

---

## Directory Structure

```
mcp-gateway/
├── cmd/
│   └── gateway/
│       └── main.go             # entrypoint: wire all components, start HTTP server
├── config/
│   └── config.go               # startup config (env vars + optional YAML)
├── store/
│   ├── store.go                # Store interface
│   ├── jsonstore.go            # JSON file implementation (atomic writes)
│   └── models.go               # SpecRecord, OperationRecord, ToolStats structs
├── spec/
│   ├── parser.go               # parse + validate uploaded OAI3 spec
│   ├── resolver.go             # $ref resolution
│   └── extractor.go            # operations → ToolDefinitions
├── registry/
│   └── registry.go             # in-memory tool store, thread-safe, rebuild logic
├── mcp/
│   ├── sse.go                  # SSE session manager + heartbeat
│   ├── http.go                 # Stateless HTTP transport handler
│   ├── handlers.go             # initialize, tools/list, tools/call, ping (shared)
│   └── types.go                # MCP JSON-RPC types
├── proxy/
│   ├── builder.go              # ToolCall args → *http.Request
│   ├── proxy.go                # execute + pool management
│   └── mapper.go               # http.Response → MCP content
├── auth/
│   ├── auth.go                 # Authenticator interface
│   ├── apikey.go
│   ├── bearer.go
│   ├── basic.go
│   ├── oauth2.go
│   └── encrypt.go              # AES-GCM encrypt/decrypt for stored credentials
├── admin/
│   ├── router.go               # /_api/* route registration
│   ├── specs.go                # spec CRUD handlers
│   ├── operations.go           # operation enable/disable handlers
│   └── stats.go                # dashboard + per-tool stats handlers
├── telemetry/
│   ├── logger.go               # slog setup
│   └── metrics.go              # Prometheus collectors
├── ui/                         # React TypeScript app
│   ├── src/
│   │   ├── main.tsx
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Specs.tsx
│   │   │   ├── SpecDetail.tsx
│   │   │   ├── Stats.tsx
│   │   │   └── Chat.tsx        # MCP test client
│   │   ├── components/
│   │   │   ├── UploadDrawer.tsx
│   │   │   ├── OperationsTable.tsx
│   │   │   ├── StatsCard.tsx
│   │   │   ├── ChatMessage.tsx
│   │   │   └── ToolCallBlock.tsx
│   │   └── lib/
│   │       ├── api.ts          # typed admin API client
│   │       └── mcp-client.ts   # MCP SSE/HTTP client for chat page
│   ├── package.json
│   └── vite.config.ts
├── ui_embed.go                 # //go:embed ui/dist/* → served at /_ui
├── data/                       # default data directory (gitignored)
│   ├── specs/
│   ├── operations/
│   ├── auth/
│   └── stats/
├── go.mod
├── go.sum
├── gateway.yaml                # example startup config
├── PLAN.md
└── README.md
```

---

## Implementation Phases

### Phase 1 — Foundation
- Project structure scaffold
- Startup config (env vars + YAML)
- JSON file store (atomic writes, all CRUD methods)
- OpenAPI 3 spec parser (YAML/JSON, $ref resolution, operation extraction)
- In-memory tool registry (thread-safe, rebuild from store)

### Phase 2 — MCP Core (SSE + HTTP)
- MCP protocol types (JSON-RPC 2.0)
- Shared protocol handlers (initialize, tools/list, tools/call stub, ping)
- SSE session manager (connect, heartbeat, disconnect, list_changed push)
- HTTP stateless transport (POST /mcp/http)

### Phase 3 — Proxy Pipeline
- Request builder (path params, query, body, Content-Type)
- HTTP proxy with connection pooling + configurable timeout
- Response mapper (JSON / text / binary / truncation)
- **Auth passthrough**: capture `Authorization` + configured headers from inbound MCP request, attach to upstream call (priority over configured spec auth)
- Wire `tools/call` end-to-end with real upstream calls
- Write stats to JSON store (in-memory accumulator → flush every 5s)

### Phase 4 — Authentication
- `Authenticator` interface
- `none`, `api-key`, `bearer`, `basic` strategies
- `oauth2-client-credentials` with token refresh
- AES-GCM credential encryption/decryption (`GATEWAY_SECRET`)
- Auth resolution chain: passthrough (inbound headers) → configured spec auth → none

### Phase 5 — Admin API
- `/_api/specs` CRUD (multipart upload, parse, populate operations, rebuild registry)
- `/_api/specs/:id/operations` enable/disable (triggers registry rebuild + list_changed)
- `/_api/stats` + `/_api/stats/tools`
- `/_api/health`

### Phase 6 — Admin UI
- Vite + React + TypeScript + shadcn/ui + Tailwind setup
- Dashboard page (stats cards + activity feed)
- Specs list + Upload drawer (drag-drop, auth config)
- Spec detail + operations table with enable/disable toggles
- Stats page (table + Recharts bar chart)
- **Chat page**: settings panel (API key, model, transport), chat thread, inline tool call/result blocks, OpenAI streaming
- Embed built UI into Go binary (`ui_embed.go`)
- Dev mode: reverse proxy `/_ui` to Vite dev server when `UI_DEV_PROXY` env set

### Phase 7 — Observability & Reliability
- Structured slog logging (JSON prod / text dev)
- Prometheus metrics endpoint (`/metrics`)
- `notifications/tools/list_changed` push on registry rebuild
- Graceful shutdown (drain SSE sessions, flush stats, close store)
- Configurable timeouts, max_response_bytes

### Phase 8 — Polish & Deployment
- Docker multi-stage build (Node build UI → Go build binary → distroless)
- `docker-compose.yml` with mock upstream (WireMock or httpbin)
- README quick-start (local run + Docker)
- Integration tests (upload spec → call tool → verify proxied request; enable/disable op)

---

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Single binary, embed UI, fast HTTP |
| MCP Transport | SSE + HTTP | SSE for stateful agents; HTTP for serverless/simple |
| OpenAPI parser | `github.com/getkin/kin-openapi` | Production-grade $ref resolution |
| Persistence | JSON files in `data/` dir | Zero infra, human-readable, no CGo, easy backup |
| Credential storage | AES-GCM encrypted JSON file | Encrypted at rest, no secrets in config |
| UI framework | React + Vite + shadcn/ui + Tailwind | Fast builds, great DX, polished components |
| UI serving | `embed.FS` | Single binary deployment |
| Auth passthrough | Inbound `Authorization` forwarded to upstream | Agents act as authenticated users; gateway stays stateless wrt credentials |
| Multi-spec | All specs merged into one MCP endpoint | Single connection point for agents regardless of how many APIs are registered |
| Logging | `log/slog` (stdlib) | No extra dep, structured, fast |
| Metrics | `prometheus/client_golang` | Industry standard |

---

## Example `gateway.yaml` (startup config only)

```yaml
listen_addr: ":8080"
data_dir: "./data"
log_level: "info"
max_response_bytes: 1048576   # 1 MB
# GATEWAY_SECRET env var required for credential encryption
# UI_DEV_PROXY env var: set to http://localhost:5173 for frontend dev mode
```
