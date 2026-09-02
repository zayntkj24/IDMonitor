import { useState, useEffect } from 'react'
import { securityApi } from '../lib/api'
import { Lock } from 'lucide-react'

export default function FIMPage() {
  const [events, setEvents] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => { securityApi.fimEvents().then(setEvents).finally(() => setLoading(false)) }, [])

  const typeBadge: Record<string, string> = {
    CREATED: 'badge-green', MODIFIED: 'badge-yellow', DELETED: 'badge-red',
    PERMISSION_CHANGED: 'badge-blue', OWNER_CHANGED: 'badge-purple',
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">File Integrity Monitoring</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : events.length === 0 ? (
        <div className="card text-center py-12">
          <Lock size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No FIM events</h3>
          <p className="text-gray-400 mt-1">Configure FIM rules and install agents to detect file changes</p>
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">Type</th>
              <th className="pb-3 font-medium">File Path</th>
              <th className="pb-3 font-medium">Old Hash</th>
              <th className="pb-3 font-medium">New Hash</th>
              <th className="pb-3 font-medium">Detected At</th>
            </tr></thead>
            <tbody>
              {events.map(e => (
                <tr key={e.id} className="table-row">
                  <td className="py-3"><span className={`badge ${typeBadge[e.event_type] || 'badge-gray'}`}>{e.event_type}</span></td>
                  <td className="py-3 font-mono text-sm">{e.file_path}</td>
                  <td className="py-3 text-xs text-gray-500 font-mono">{e.old_hash || '-'}</td>
                  <td className="py-3 text-xs text-gray-500 font-mono">{e.new_hash || '-'}</td>
                  <td className="py-3 text-sm">{new Date(e.detected_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
