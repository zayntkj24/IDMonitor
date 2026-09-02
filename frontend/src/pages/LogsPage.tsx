import { useState, useEffect } from 'react'
import { logsApi } from '../lib/api'
import { FileText, Search } from 'lucide-react'

export default function LogsPage() {
  const [logs, setLogs] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [level, setLevel] = useState('')
  const [search, setSearch] = useState('')

  useEffect(() => { loadLogs() }, [level, search])

  const loadLogs = () => {
    setLoading(true)
    const params: Record<string, string> = {}
    if (level) params.level = level
    if (search) params.q = search
    logsApi.list(params).then(setLogs).finally(() => setLoading(false))
  }

  const levelColors: Record<string, string> = {
    ERROR: 'text-red-400', WARN: 'text-yellow-400', INFO: 'text-blue-400', DEBUG: 'text-gray-400',
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Logs</h1>
      <div className="flex gap-4">
        <div className="relative flex-1">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <input className="input pl-10" placeholder="Search logs..." value={search} onChange={e => setSearch(e.target.value)} />
        </div>
        <select className="input w-auto" value={level} onChange={e => setLevel(e.target.value)}>
          <option value="">All Levels</option>
          <option value="ERROR">Error</option>
          <option value="WARN">Warning</option>
          <option value="INFO">Info</option>
          <option value="DEBUG">Debug</option>
        </select>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : logs.length === 0 ? (
        <div className="card text-center py-12">
          <FileText size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No logs</h3>
          <p className="text-gray-400 mt-1">Logs will appear when agents report data</p>
        </div>
      ) : (
        <div className="card">
          <div className="space-y-1 font-mono text-sm max-h-[600px] overflow-y-auto">
            {logs.map(log => (
              <div key={log.id} className="flex gap-3 py-1.5 border-b border-gray-800/50">
                <span className="text-gray-500 shrink-0">{new Date(log.timestamp).toLocaleString()}</span>
                <span className={`shrink-0 font-medium ${levelColors[log.level] || 'text-gray-400'}`} style={{ minWidth: '50px' }}>{log.level}</span>
                <span className="text-gray-200 break-all">{log.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
