import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getToolStats, type ToolStats } from '../lib/api'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
import { ChevronUp, ChevronDown } from 'lucide-react'

type SortKey = keyof ToolStats
type SortDir = 'asc' | 'desc'

export default function Stats() {
  const [sortKey, setSortKey] = useState<SortKey>('call_count')
  const [sortDir, setSortDir] = useState<SortDir>('desc')

  const { data: stats, isLoading, error } = useQuery({
    queryKey: ['toolStats'],
    queryFn: getToolStats,
    refetchInterval: 30000,
  })

  const sorted = useMemo(() => {
    if (!stats) return []
    return [...stats].sort((a, b) => {
      const av = a[sortKey] as number | string
      const bv = b[sortKey] as number | string
      if (typeof av === 'number' && typeof bv === 'number') {
        return sortDir === 'asc' ? av - bv : bv - av
      }
      return sortDir === 'asc' ? String(av).localeCompare(String(bv)) : String(bv).localeCompare(String(av))
    })
  }, [stats, sortKey, sortDir])

  const chartData = useMemo(() =>
    [...(stats ?? [])].sort((a, b) => b.call_count - a.call_count).slice(0, 10).map(s => ({
      name: s.operation_id.length > 20 ? s.operation_id.slice(0, 20) + '…' : s.operation_id,
      calls: s.call_count,
      errors: s.error_count,
    })), [stats])

  const handleSort = (key: SortKey) => {
    if (key === sortKey) setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    else { setSortKey(key); setSortDir('desc') }
  }

  const SortIcon = ({ k }: { k: SortKey }) => sortKey === k
    ? (sortDir === 'asc' ? <ChevronUp className="w-3 h-3 inline ml-1" /> : <ChevronDown className="w-3 h-3 inline ml-1" />)
    : null

  if (isLoading) return <div className="p-6 text-gray-400">Loading stats...</div>
  if (error) return <div className="p-6 text-red-400">Failed to load stats</div>

  return (
    <div className="p-6">
      <h2 className="text-2xl font-bold text-white mb-6">Tool Stats</h2>

      {chartData.length > 0 && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
          <h3 className="text-sm font-medium text-gray-400 mb-4">Top 10 Tools by Call Count</h3>
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={chartData} margin={{ top: 0, right: 0, left: -20, bottom: 40 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis dataKey="name" tick={{ fill: '#9CA3AF', fontSize: 11 }} angle={-35} textAnchor="end" interval={0} />
              <YAxis tick={{ fill: '#9CA3AF', fontSize: 11 }} />
              <Tooltip contentStyle={{ backgroundColor: '#1F2937', border: '1px solid #374151', color: '#F9FAFB' }} />
              <Bar dataKey="calls" fill="#3B82F6" radius={[4, 4, 0, 0] as unknown as number} />
              <Bar dataKey="errors" fill="#EF4444" radius={[4, 4, 0, 0] as unknown as number} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {!stats || stats.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-500">
          No tool stats yet. Make some MCP calls to see data here.
        </div>
      ) : (
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-800 text-left">
                {([
                  ['operation_id', 'Operation ID'],
                  ['call_count', 'Calls'],
                  ['error_count', 'Errors'],
                  ['total_latency_ms', 'Avg Latency'],
                  ['last_called_at', 'Last Called'],
                ] as [SortKey, string][]).map(([key, label]) => (
                  <th key={key} onClick={() => handleSort(key)}
                    className="px-4 py-3 text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-white">
                    {label}<SortIcon k={key} />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {sorted.map(s => {
                const errPct = s.call_count > 0 ? ((s.error_count / s.call_count) * 100).toFixed(1) : '0.0'
                const avgLatency = s.call_count > 0 ? Math.round(s.total_latency_ms / s.call_count) : 0
                return (
                  <tr key={s.operation_id} className="hover:bg-gray-800/50 transition-colors">
                    <td className="px-4 py-3 text-gray-300 text-sm font-mono">{s.operation_id}</td>
                    <td className="px-4 py-3 text-white">{s.call_count}</td>
                    <td className="px-4 py-3">
                      <span className={s.error_count > 0 ? 'text-red-400' : 'text-gray-400'}>{s.error_count}</span>
                      <span className="text-gray-500 text-xs ml-1">({errPct}%)</span>
                    </td>
                    <td className="px-4 py-3 text-gray-400 text-sm">{avgLatency}ms</td>
                    <td className="px-4 py-3 text-gray-400 text-sm">
                      {s.last_called_at ? new Date(s.last_called_at).toLocaleString() : '—'}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
