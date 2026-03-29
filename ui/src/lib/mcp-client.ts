export type Transport = 'sse' | 'http'

export interface MCPTool {
  name: string
  description: string
  inputSchema: Record<string, unknown>
}

export interface MCPClient {
  listTools(): Promise<MCPTool[]>
  callTool(name: string, args: Record<string, unknown>): Promise<string>
  disconnect(): void
}

async function postMCP(url: string, body: unknown): Promise<unknown> {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`MCP error: ${res.statusText}`)
  return res.json()
}

let msgIdCounter = 1

function nextId() { return msgIdCounter++ }

export async function createMCPClient(transport: Transport): Promise<MCPClient> {
  if (transport === 'http') {
    return {
      listTools: async () => {
        const res = await postMCP('/mcp/http', { jsonrpc: '2.0', id: nextId(), method: 'tools/list', params: {} }) as { result?: { tools?: { name: string; description?: string; inputSchema?: Record<string, unknown> }[] } }
        return (res.result?.tools ?? []).map((t) => ({
          name: t.name,
          description: t.description ?? '',
          inputSchema: t.inputSchema ?? {},
        }))
      },
      callTool: async (name, args) => {
        const res = await postMCP('/mcp/http', { jsonrpc: '2.0', id: nextId(), method: 'tools/call', params: { name, arguments: args } }) as { result?: { content?: { type: string; text?: string }[] } }
        const content = res.result?.content ?? []
        if (content.length === 0) return ''
        if (content[0].type === 'text') return content[0].text ?? ''
        return JSON.stringify(content)
      },
      disconnect: () => {},
    }
  }

  // SSE transport
  return new Promise((resolve, reject) => {
    const es = new EventSource('/mcp/sse', { withCredentials: true })
    let messageUrl = ''
    const pendingRequests = new Map<number, (result: unknown) => void>()
    const pendingErrors = new Map<number, (err: Error) => void>()

    es.addEventListener('endpoint', (e: MessageEvent) => {
      messageUrl = e.data as string
      if (!messageUrl.startsWith('http')) {
        messageUrl = window.location.origin + messageUrl
      }

      // Initialize session
      const id = nextId()
      void fetch(messageUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ jsonrpc: '2.0', id, method: 'initialize', params: { protocolVersion: '2024-11-05', capabilities: {}, clientInfo: { name: 'mcp-gateway-ui', version: '1.0' } } }),
      })
    })

    es.addEventListener('message', (e: MessageEvent) => {
      try {
        const msg = JSON.parse(e.data as string) as { id?: number; error?: { message?: string }; result?: { protocolVersion?: string }; method?: string }
        if (msg.id !== undefined) {
          const res = pendingRequests.get(msg.id)
          const rej = pendingErrors.get(msg.id)
          if (msg.error) {
            rej?.(new Error(msg.error.message ?? 'MCP error'))
          } else {
            res?.(msg)
          }
          pendingRequests.delete(msg.id)
          pendingErrors.delete(msg.id)
        }
        // After initialize, resolve the client
        if (msg.method === 'initialized' || (msg.id !== undefined && msg.result?.protocolVersion)) {
          resolve(client)
        }
      } catch {}
    })

    es.onerror = () => {
      reject(new Error('SSE connection failed'))
    }

    // Resolve after a short delay even if no explicit initialized event
    setTimeout(() => resolve(client), 2000)

    const sendRequest = (method: string, params: unknown): Promise<unknown> => {
      return new Promise((res, rej) => {
        const id = nextId()
        pendingRequests.set(id, res)
        pendingErrors.set(id, rej)
        void fetch(messageUrl, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ jsonrpc: '2.0', id, method, params }),
        })
        setTimeout(() => {
          if (pendingRequests.has(id)) {
            pendingRequests.delete(id)
            pendingErrors.delete(id)
            rej(new Error('Request timeout'))
          }
        }, 30000)
      })
    }

    const client: MCPClient = {
      listTools: async () => {
        const result = await sendRequest('tools/list', {}) as { tools?: { name: string; description?: string; inputSchema?: Record<string, unknown> }[] } | null
        return (result?.tools ?? []).map((t) => ({
          name: t.name,
          description: t.description ?? '',
          inputSchema: t.inputSchema ?? {},
        }))
      },
      callTool: async (name, args) => {
        const result = await sendRequest('tools/call', { name, arguments: args }) as { content?: { type: string; text?: string }[] } | null
        const content = result?.content ?? []
        if (content.length === 0) return ''
        if (content[0].type === 'text') return content[0].text ?? ''
        return JSON.stringify(content)
      },
      disconnect: () => es.close(),
    }
  })
}
