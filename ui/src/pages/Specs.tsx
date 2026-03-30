import { useState, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { listSpecs, uploadSpec, deleteSpec, type SpecRecord, type UploadSpecPayload } from '../lib/api'
import { Plus, Trash2, Eye, X, Upload, Shield, Cookie } from 'lucide-react'
import { cn } from '../lib/utils'
import * as yaml from 'js-yaml'

interface SpecMeta { title?: string; serverUrl?: string }

function extractSpecMeta(text: string, filename: string): SpecMeta {
  try {
    const doc = (filename.endsWith('.json') ? JSON.parse(text) : yaml.load(text)) as Record<string, unknown>
    const title = (doc?.info as Record<string, unknown>)?.title as string | undefined
    const servers = doc?.servers as Array<{ url?: string }> | undefined
    const serverUrl = servers?.[0]?.url
    return { title, serverUrl }
  } catch {
    return {}
  }
}

function UploadDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [upstreamUrl, setUpstreamUrl] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [passthroughAuth, setPassthroughAuth] = useState(false)
  const [passthroughCookies, setPassthroughCookies] = useState(false)
  const [passthroughHeadersRaw, setPassthroughHeadersRaw] = useState('')
  const [dragOver, setDragOver] = useState(false)
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const mutation = useMutation({
    mutationFn: ({ file, payload }: { file: File; payload: UploadSpecPayload }) =>
      uploadSpec(file, payload),
    onSuccess: (data) => {
      setSuccessMsg(`Spec "${data.name}" uploaded successfully.`)
      void queryClient.invalidateQueries({ queryKey: ['specs'] })
      setTimeout(() => { setSuccessMsg(''); onClose() }, 2000)
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const applyFileMeta = useCallback((f: File) => {
    setFile(f)
    const reader = new FileReader()
    reader.onload = (e) => {
      const text = e.target?.result as string
      const meta = extractSpecMeta(text, f.name)
      if (meta.title && !name) setName(meta.title)
      if (meta.serverUrl && !upstreamUrl) setUpstreamUrl(meta.serverUrl)
    }
    reader.readAsText(f)
  }, [name, upstreamUrl])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(false)
    const f = e.dataTransfer.files[0]
    if (f) applyFileMeta(f)
  }, [applyFileMeta])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setErrorMsg('')
    if (!file) { setErrorMsg('Please select a spec file'); return }
    const headers = passthroughHeadersRaw.split(',').map(h => h.trim()).filter(Boolean)
    mutation.mutate({
      file,
      payload: { name, upstream_url: upstreamUrl, passthrough_auth: passthroughAuth, passthrough_cookies: passthroughCookies, passthrough_headers: headers, mtls_enabled: false },
    })
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div className="relative w-full max-w-md bg-gray-900 border-l border-gray-700 shadow-2xl flex flex-col h-full overflow-y-auto">
        <div className="flex items-center justify-between p-5 border-b border-gray-800">
          <h3 className="text-lg font-semibold text-white">Upload New Spec</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-white"><X className="w-5 h-5" /></button>
        </div>
        <form onSubmit={handleSubmit} className="flex-1 p-5 space-y-4">
          <div>
            <label className="text-sm text-gray-400 block mb-1">Display Name</label>
            <input value={name} onChange={e => setName(e.target.value)}
              placeholder="Auto-filled from spec info.title"
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500" />
          </div>
          <div>
            <label className="text-sm text-gray-400 block mb-1">Upstream Base URL</label>
            <input value={upstreamUrl} onChange={e => setUpstreamUrl(e.target.value)} placeholder="Auto-filled from spec servers[0]"
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500" />
          </div>
          <div>
            <label className="text-sm text-gray-400 block mb-1">OpenAPI Spec File</label>
            <div
              onDrop={handleDrop}
              onDragOver={e => { e.preventDefault(); setDragOver(true) }}
              onDragLeave={() => setDragOver(false)}
              className={cn('border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors',
                dragOver ? 'border-blue-500 bg-blue-900/20' : 'border-gray-700 hover:border-gray-500')}
              onClick={() => document.getElementById('spec-file-input')?.click()}
            >
              <Upload className="w-8 h-8 text-gray-500 mx-auto mb-2" />
              {file ? (
                <p className="text-sm text-green-400">{file.name}</p>
              ) : (
                <p className="text-sm text-gray-400">Drop .yaml, .yml, or .json<br />or click to browse</p>
              )}
              <input id="spec-file-input" type="file" accept=".yaml,.yml,.json" className="hidden"
                onChange={e => e.target.files?.[0] && applyFileMeta(e.target.files[0])} />
            </div>
          </div>
          <div className="space-y-3">
            <h4 className="text-sm font-medium text-gray-300">Passthrough Settings</h4>
            <label className="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" checked={passthroughAuth} onChange={e => setPassthroughAuth(e.target.checked)}
                className="w-4 h-4 rounded border-gray-600 bg-gray-800" />
              <span className="text-sm text-gray-300 flex items-center gap-1.5"><Shield className="w-4 h-4 text-blue-400" /> Forward Authorization header</span>
            </label>
            <label className="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" checked={passthroughCookies} onChange={e => setPassthroughCookies(e.target.checked)}
                className="w-4 h-4 rounded border-gray-600 bg-gray-800" />
              <span className="text-sm text-gray-300 flex items-center gap-1.5"><Cookie className="w-4 h-4 text-blue-400" /> Forward Cookie header</span>
            </label>
            <div>
              <label className="text-sm text-gray-400 block mb-1">Additional headers to forward (comma-separated)</label>
              <input value={passthroughHeadersRaw} onChange={e => setPassthroughHeadersRaw(e.target.value)}
                placeholder="X-Custom-Header, X-Tenant-ID"
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500" />
            </div>
          </div>
          {errorMsg && <div className="text-red-400 text-sm bg-red-900/20 border border-red-800 rounded p-2">{errorMsg}</div>}
          {successMsg && <div className="text-green-400 text-sm bg-green-900/20 border border-green-800 rounded p-2">{successMsg}</div>}
          <button type="submit" disabled={mutation.isPending}
            className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white rounded-lg py-2.5 text-sm font-medium transition-colors">
            {mutation.isPending ? 'Uploading...' : 'Upload Spec'}
          </button>
        </form>
      </div>
    </div>
  )
}

export default function Specs() {
  const queryClient = useQueryClient()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [deleteId, setDeleteId] = useState<string | null>(null)

  const { data: specs, isLoading, error } = useQuery({
    queryKey: ['specs'],
    queryFn: listSpecs,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteSpec,
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['specs'] }); setDeleteId(null) },
  })

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-white">Specs</h2>
        <button onClick={() => setDrawerOpen(true)}
          className="flex items-center gap-2 bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors">
          <Plus className="w-4 h-4" /> Upload New Spec
        </button>
      </div>

      {isLoading && <div className="text-gray-400">Loading specs...</div>}
      {error && <div className="text-red-400">Failed to load specs</div>}

      {specs && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-800 text-left">
                <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wider">Name</th>
                <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wider">Upstream URL</th>
                <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wider">Passthrough</th>
                <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wider">Uploaded</th>
                <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {specs.length === 0 && (
                <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-500">No specs yet. Upload one to get started.</td></tr>
              )}
              {specs.map((spec: SpecRecord) => (
                <tr key={spec.id} className="hover:bg-gray-800/50 transition-colors">
                  <td className="px-4 py-3">
                    <span className="text-white font-medium">{spec.name || spec.id.slice(0, 8)}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-gray-400 text-sm font-mono truncate max-w-xs block">{spec.upstream_url}</span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1">
                      {spec.passthrough_auth && <span className="px-1.5 py-0.5 bg-blue-900/50 text-blue-300 text-xs rounded border border-blue-800">Auth</span>}
                      {spec.passthrough_cookies && <span className="px-1.5 py-0.5 bg-purple-900/50 text-purple-300 text-xs rounded border border-purple-800">Cookie</span>}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-gray-400 text-sm">{new Date(spec.created_at).toLocaleDateString()}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Link to="/specs/$specId" params={{ specId: spec.id }}
                        className="p-1.5 text-gray-400 hover:text-blue-400 transition-colors rounded hover:bg-gray-700">
                        <Eye className="w-4 h-4" />
                      </Link>
                      <button onClick={() => setDeleteId(spec.id)}
                        className="p-1.5 text-gray-400 hover:text-red-400 transition-colors rounded hover:bg-gray-700">
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <UploadDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />

      {deleteId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60" onClick={() => setDeleteId(null)} />
          <div className="relative bg-gray-900 border border-gray-700 rounded-xl p-6 max-w-sm w-full mx-4">
            <h3 className="text-lg font-semibold text-white mb-2">Delete Spec</h3>
            <p className="text-gray-400 text-sm mb-5">Are you sure? This will remove the spec and all its operations.</p>
            <div className="flex gap-3">
              <button onClick={() => setDeleteId(null)} className="flex-1 bg-gray-800 hover:bg-gray-700 text-white rounded-lg py-2 text-sm transition-colors">Cancel</button>
              <button onClick={() => deleteMutation.mutate(deleteId)}
                disabled={deleteMutation.isPending}
                className="flex-1 bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white rounded-lg py-2 text-sm transition-colors">
                {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
