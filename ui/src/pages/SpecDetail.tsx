import { useState, useMemo } from 'react'
import { useParams } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getSpec, toggleOperation, type OperationRecord } from '../lib/api'
import MethodBadge from '../components/MethodBadge'
import { ArrowLeft, Copy, Check, Search, ChevronDown } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { cn } from '../lib/utils'

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button onClick={() => { void navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000) }}
      className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-300 text-sm rounded-lg transition-colors">
      {copied ? <Check className="w-3.5 h-3.5 text-green-400" /> : <Copy className="w-3.5 h-3.5" />}
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

export default function SpecDetail() {
  const { specId } = useParams({ from: '/specs/$specId' })
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [tagFilter, setTagFilter] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['spec', specId],
    queryFn: () => getSpec(specId),
  })

  const toggleMutation = useMutation({
    mutationFn: ({ opId, enabled }: { opId: string; enabled: boolean }) =>
      toggleOperation(specId, opId, enabled),
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['spec', specId] }) },
  })

  const spec = data?.spec
  const operations = data?.operations ?? []

  const allTags = useMemo(() => {
    const tags = new Set<string>()
    operations.forEach(op => op.tags?.forEach(t => tags.add(t)))
    return Array.from(tags)
  }, [operations])

  const filtered = useMemo(() => {
    return operations.filter(op => {
      const matchSearch = !search ||
        op.operation_id.toLowerCase().includes(search.toLowerCase()) ||
        op.path.toLowerCase().includes(search.toLowerCase()) ||
        (op.summary ?? '').toLowerCase().includes(search.toLowerCase())
      const matchTag = !tagFilter || op.tags?.includes(tagFilter)
      return matchSearch && matchTag
    })
  }, [operations, search, tagFilter])

  const handleBulkToggle = async (enabled: boolean) => {
    for (const op of filtered) {
      await toggleOperation(specId, op.id, enabled)
    }
    void queryClient.invalidateQueries({ queryKey: ['spec', specId] })
  }

  const sseUrl = `${window.location.origin}/mcp/sse`
  const httpUrl = `${window.location.origin}/mcp/http`

  if (isLoading) return <div className="p-6 text-gray-400">Loading spec...</div>
  if (error) return <div className="p-6 text-red-400">Failed to load spec</div>
  if (!spec) return null

  return (
    <div className="p-6">
      <div className="flex items-center gap-3 mb-6">
        <Link to="/specs" className="text-gray-400 hover:text-white transition-colors">
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <div>
          <h2 className="text-2xl font-bold text-white">{spec.name}</h2>
          <p className="text-gray-400 text-sm font-mono">{spec.upstream_url}</p>
        </div>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 mb-6 flex flex-wrap gap-4 items-center">
        <div className="flex gap-2">
          {spec.passthrough_auth && <span className="px-2 py-1 bg-blue-900/50 text-blue-300 text-xs rounded border border-blue-800">Passthrough Auth</span>}
          {spec.passthrough_cookies && <span className="px-2 py-1 bg-purple-900/50 text-purple-300 text-xs rounded border border-purple-800">Passthrough Cookies</span>}
          {spec.passthrough_headers?.length > 0 && (
            <span className="px-2 py-1 bg-gray-700 text-gray-300 text-xs rounded">+{spec.passthrough_headers.length} headers</span>
          )}
        </div>
        <div className="flex gap-2 ml-auto">
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-400">SSE:</span>
            <CopyButton text={sseUrl} />
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-400">HTTP:</span>
            <CopyButton text={httpUrl} />
          </div>
        </div>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <div className="p-4 border-b border-gray-800 flex flex-wrap gap-3 items-center">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
            <input value={search} onChange={e => setSearch(e.target.value)}
              placeholder="Search operations..."
              className="w-full bg-gray-800 border border-gray-700 rounded-lg pl-9 pr-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500" />
          </div>
          {allTags.length > 0 && (
            <div className="relative">
              <select value={tagFilter} onChange={e => setTagFilter(e.target.value)}
                className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500 appearance-none pr-8">
                <option value="">All tags</option>
                {allTags.map(t => <option key={t} value={t}>{t}</option>)}
              </select>
              <ChevronDown className="absolute right-2 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500 pointer-events-none" />
            </div>
          )}
          <button onClick={() => { void handleBulkToggle(true) }}
            className="px-3 py-2 bg-green-700 hover:bg-green-600 text-white text-sm rounded-lg transition-colors">Enable All</button>
          <button onClick={() => { void handleBulkToggle(false) }}
            className="px-3 py-2 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded-lg transition-colors">Disable All</button>
        </div>

        <table className="w-full">
          <thead>
            <tr className="border-b border-gray-800 text-left">
              <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase">Method</th>
              <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase">Path</th>
              <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase">Operation ID</th>
              <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase">Summary</th>
              <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase">Tags</th>
              <th className="px-4 py-3 text-xs font-medium text-gray-400 uppercase">Enabled</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {filtered.map((op: OperationRecord) => (
              <tr key={op.id} className="hover:bg-gray-800/50 transition-colors">
                <td className="px-4 py-3"><MethodBadge method={op.method} /></td>
                <td className="px-4 py-3 text-gray-300 text-sm font-mono">{op.path}</td>
                <td className="px-4 py-3 text-gray-300 text-sm">{op.operation_id}</td>
                <td className="px-4 py-3 text-gray-400 text-sm max-w-xs truncate">{op.summary}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {op.tags?.map(t => (
                      <span key={t} className="px-1.5 py-0.5 bg-gray-700 text-gray-300 text-xs rounded">{t}</span>
                    ))}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <button
                    onClick={() => toggleMutation.mutate({ opId: op.id, enabled: !op.enabled })}
                    className={cn('relative inline-flex h-5 w-9 items-center rounded-full transition-colors',
                      op.enabled ? 'bg-blue-600' : 'bg-gray-700'
                    )}
                  >
                    <span className={cn('inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform',
                      op.enabled ? 'translate-x-4' : 'translate-x-1'
                    )} />
                  </button>
                </td>
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-500">No operations found</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
