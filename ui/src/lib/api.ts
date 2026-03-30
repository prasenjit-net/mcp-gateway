const BASE = '/_api'

export interface SpecRecord {
  id: string
  name: string
  upstream_url: string
  spec_raw: string
  passthrough_auth: boolean
  passthrough_cookies: boolean
  passthrough_headers: string[]
  mtls_enabled: boolean
  created_at: string
  updated_at: string
}

export interface OperationRecord {
  id: string
  spec_id: string
  operation_id: string
  method: string
  path: string
  summary: string
  description: string
  tags: string[]
  enabled: boolean
}

export interface ToolStats {
  operation_id: string
  call_count: number
  error_count: number
  total_latency_ms: number
  last_called_at: string
}

export interface GlobalStats {
  totalSpecs: number
  totalTools: number
  enabledTools: number
  totalCalls: number
  totalErrors: number
  activeSessions: number
}

export interface UploadSpecPayload {
  name: string
  upstream_url: string
  passthrough_auth: boolean
  passthrough_cookies: boolean
  passthrough_headers: string[]
  mtls_enabled: boolean
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let msg = res.statusText
    try { const j = await res.json(); msg = j.error || msg } catch {}
    throw new Error(msg)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export async function listSpecs(): Promise<SpecRecord[]> {
  const res = await fetch(`${BASE}/specs`)
  return handleResponse(res)
}

export async function getSpec(id: string): Promise<{spec: SpecRecord, operations: OperationRecord[]}> {
  const res = await fetch(`${BASE}/specs/${id}`)
  return handleResponse(res)
}

export async function uploadSpec(file: File, payload: UploadSpecPayload): Promise<SpecRecord> {
  const fd = new FormData()
  fd.append('spec', file)
  fd.append('name', payload.name)
  fd.append('upstream_url', payload.upstream_url)
  fd.append('passthrough_auth', String(payload.passthrough_auth))
  fd.append('passthrough_cookies', String(payload.passthrough_cookies))
  fd.append('passthrough_headers', JSON.stringify(payload.passthrough_headers))
  fd.append('mtls_enabled', String(payload.mtls_enabled))
  const res = await fetch(`${BASE}/specs`, { method: 'POST', body: fd })
  return handleResponse(res)
}

export async function updateSpec(id: string, payload: Partial<UploadSpecPayload>): Promise<SpecRecord> {
  const res = await fetch(`${BASE}/specs/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  return handleResponse(res)
}

export async function deleteSpec(id: string): Promise<void> {
  const res = await fetch(`${BASE}/specs/${id}`, { method: 'DELETE' })
  return handleResponse(res)
}

export async function listOperations(specId: string): Promise<OperationRecord[]> {
  const res = await fetch(`${BASE}/specs/${specId}/operations`)
  return handleResponse(res)
}

export async function toggleOperation(specId: string, opId: string, enabled: boolean): Promise<OperationRecord> {
  const res = await fetch(`${BASE}/specs/${specId}/operations/${opId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
  return handleResponse(res)
}

export async function getGlobalStats(): Promise<GlobalStats> {
  const res = await fetch(`${BASE}/stats`)
  return handleResponse(res)
}

export async function getToolStats(): Promise<ToolStats[]> {
  const res = await fetch(`${BASE}/stats/tools`)
  return handleResponse(res)
}

export async function getHealth(): Promise<{status: string, uptime: string, version: string}> {
  const res = await fetch(`${BASE}/health`)
  return handleResponse(res)
}
