import { useState, useEffect } from 'react'
import { alertsApi } from '../lib/api'
import { AlertTriangle, Check } from 'lucide-react'

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<any[]>([])
  const [stats, setStats] = useState<any>({})
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState('')

  useEffect(() => {
    const params: Record<string, string> = {}
    if (statusFilter) params.status = statusFilter
    Promise.all([alertsApi.list(params), alertsApi.stats()])
      .then(([a, s]) => { setAlerts(a.alerts || []); setStats(s) })
      .finally(() => setLoading(false))
  }, [statusFilter])

  const handleAck = async (id: string) => {
    await alertsApi.acknowledge(id)
    setAlerts(alerts.map(a => a.id === id ? { ...a, status: 'ACKNOWLEDGED' } : a))
  }

  const handleResolve = async (id: string) => {
    await alertsApi.resolve(id)
    setAlerts(alerts.map(a => a.id === id ? { ...a, status: 'RESOLVED' } : a))
  }

  const sevBadge: Record<string, string> = {
    CRITICAL: 'badge-red', HIGH: 'badge-red', MEDIUM: 'badge-yellow', LOW: 'badge-blue', INFO: 'badge-gray',
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Alerts</h1>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div className="card text-center"><p className="text-2xl font-bold">{stats.open || 0}</p><p className="text-sm text-gray-400">Open</p></div>
        <div className="card text-center"><p className="text-2xl font-bold">{stats.acknowledged || 0}</p><p className="text-sm text-gray-400">Acknowledged</p></div>
        <div className="card text-center"><p className="text-2xl font-bold text-red-400">{stats.critical || 0}</p><p className="text-sm text-gray-400">Critical</p></div>
        <div className="card text-center"><p className="text-2xl font-bold text-orange-400">{stats.high || 0}</p><p className="text-sm text-gray-400">High</p></div>
      </div>

      <select className="input w-auto" value={statusFilter} onChange={e => setStatusFilter(e.target.value)}>
        <option value="">All Status</option>
        <option value="OPEN">Open</option>
        <option value="ACKNOWLEDGED">Acknowledged</option>
        <option value="RESOLVED">Resolved</option>
      </select>

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : alerts.length === 0 ? (
        <div className="card text-center py-12">
          <AlertTriangle size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No alerts</h3>
        </div>
      ) : (
        <div className="space-y-3">
          {alerts.map(alert => (
            <div key={alert.id} className="card flex items-start justify-between">
              <div>
                <div className="flex items-center gap-2">
                  <span className={`badge ${sevBadge[alert.severity]}`}>{alert.severity}</span>
                  <span className={`badge ${alert.status === 'OPEN' ? 'badge-red' : alert.status === 'ACKNOWLEDGED' ? 'badge-yellow' : 'badge-green'}`}>{alert.status}</span>
                  <h3 className="font-medium">{alert.title}</h3>
                </div>
                {alert.description && <p className="text-sm text-gray-400 mt-1">{alert.description}</p>}
                <p className="text-xs text-gray-500 mt-2">{new Date(alert.created_at).toLocaleString()} | Source: {alert.source || 'System'}</p>
              </div>
              <div className="flex gap-2">
                {alert.status === 'OPEN' && <button onClick={() => handleAck(alert.id)} className="btn-secondary btn-sm"><Check size={14} /> Ack</button>}
                {alert.status !== 'RESOLVED' && <button onClick={() => handleResolve(alert.id)} className="btn-primary btn-sm">Resolve</button>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
