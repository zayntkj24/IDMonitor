import { useState, useEffect } from 'react'
import { scannerApi } from '../lib/api'
import { Clock } from 'lucide-react'

export default function ScanHistoryPage() {
  const [scans, setScans] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => { scannerApi.scans().then(setScans).finally(() => setLoading(false)) }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Scan History</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : scans.length === 0 ? (
        <div className="card text-center py-12">
          <Clock size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No scan history</h3>
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">Target</th>
              <th className="pb-3 font-medium">Status</th>
              <th className="pb-3 font-medium">Hosts</th>
              <th className="pb-3 font-medium">Ports</th>
              <th className="pb-3 font-medium">Duration</th>
              <th className="pb-3 font-medium">Created</th>
            </tr></thead>
            <tbody>
              {scans.map(s => (
                <tr key={s.id} className="table-row">
                  <td className="py-3 font-medium">{s.target}</td>
                  <td className="py-3"><span className={`badge ${s.status === 'COMPLETED' ? 'badge-green' : s.status === 'FAILED' ? 'badge-red' : 'badge-gray'}`}>{s.status}</span></td>
                  <td className="py-3 text-sm">{s.hosts_discovered}</td>
                  <td className="py-3 text-sm">{s.ports_discovered}</td>
                  <td className="py-3 text-sm">{s.duration_ms ? `${(s.duration_ms / 1000).toFixed(1)}s` : '-'}</td>
                  <td className="py-3 text-sm text-gray-400">{new Date(s.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
