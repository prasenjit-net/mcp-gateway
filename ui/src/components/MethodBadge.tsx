import { cn } from '../lib/utils'

const METHOD_COLORS: Record<string, string> = {
  GET: 'bg-green-900 text-green-300 border border-green-700',
  POST: 'bg-blue-900 text-blue-300 border border-blue-700',
  PUT: 'bg-amber-900 text-amber-300 border border-amber-700',
  PATCH: 'bg-orange-900 text-orange-300 border border-orange-700',
  DELETE: 'bg-red-900 text-red-300 border border-red-700',
}

export default function MethodBadge({ method }: { method: string }) {
  return (
    <span className={cn('px-1.5 py-0.5 text-xs font-mono font-bold rounded', METHOD_COLORS[method.toUpperCase()] ?? 'bg-gray-700 text-gray-300')}>
      {method.toUpperCase()}
    </span>
  )
}
