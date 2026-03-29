import { useQuery } from '@tanstack/react-query'
import { getGlobalStats } from '../lib/api'
import StatsCard from '../components/StatsCard'
import { Database, Wrench, CheckCircle, Activity, AlertCircle, Users, Copy, Check } from 'lucide-react'
import { useState } from 'react'

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      onClick={() => { void navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000) }}
      className="ml-2 p-1 text-gray-400 hover:text-white transition-colors"
    >
      {copied ? <Check className="w-4 h-4 text-green-400" /> : <Copy className="w-4 h-4" />}
    </button>
  )
}

export default function Dashboard() {
  const { data: stats, isLoading, error } = useQuery({
    queryKey: ['globalStats'],
    queryFn: getGlobalStats,
    refetchInterval: 10000,
  })

  const sseUrl = `${window.location.origin}/mcp/sse`
  const httpUrl = `${window.location.origin}/mcp/http`
  const errorRate = stats && stats.totalCalls > 0
    ? ((stats.totalErrors / stats.totalCalls) * 100).toFixed(1) + '%'
    : '0%'

  return (
    <div className="p-6">
      <h2 className="text-2xl font-bold text-white mb-6">Dashboard</h2>

      {isLoading && <div className="text-gray-400">Loading stats...</div>}
      {error && <div className="text-red-400">Failed to load stats</div>}

      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-3 gap-4 mb-8">
          <StatsCard title="Total Specs" value={stats.totalSpecs} icon={<Database className="w-5 h-5" />} />
          <StatsCard title="Total Tools" value={stats.totalTools} icon={<Wrench className="w-5 h-5" />} />
          <StatsCard title="Enabled Tools" value={stats.enabledTools} icon={<CheckCircle className="w-5 h-5" />} />
          <StatsCard title="Total Calls" value={stats.totalCalls} icon={<Activity className="w-5 h-5" />} />
          <StatsCard title="Error Rate" value={errorRate} subtitle={`${stats.totalErrors} errors`} icon={<AlertCircle className="w-5 h-5" />} />
          <StatsCard title="Active Sessions" value={stats.activeSessions} icon={<Users className="w-5 h-5" />} />
        </div>
      )}

      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <h3 className="text-lg font-semibold text-white mb-4">Quick Connect</h3>
        <div className="space-y-3">
          {[
            { label: 'SSE Endpoint', url: sseUrl },
            { label: 'HTTP Endpoint', url: httpUrl },
          ].map(({ label, url }) => (
            <div key={label} className="flex items-center gap-2">
              <span className="text-sm text-gray-400 w-32">{label}</span>
              <code className="flex-1 bg-gray-800 text-gray-200 px-3 py-1.5 rounded text-sm font-mono">{url}</code>
              <CopyButton text={url} />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
