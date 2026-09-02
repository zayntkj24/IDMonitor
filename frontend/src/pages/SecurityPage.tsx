import { useState, useEffect } from 'react'
import { securityApi } from '../lib/api'
import { Shield, Check } from 'lucide-react'

export default function SecurityPage() {
  const [events, setEvents] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [severity, setSeverity] = useState('')

  useEffect(() => {
    setLoading(true)
    const params: Record<string, string> = {}
    if (severity) params.severity = severity
    securityApi.events(params).then(setEvents).finally(() => setLoading(false))
  }, [severity])

  const handleAck = async (id: string) => {
    await securityApi.acknowledgeEvent(id)
    setEvents(events.map(e => e.id === id ? { ...e, acknowledged: true } : e))
  }

  const severityBadge: Record<string, string> = {
    CRITICAL: 'badge-red', HIGH: 'badge-red', MEDIUM: 'badge-yellow', LOW: 'badge-blue', INFO: 'badge-gray',
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Security Events</h1>
      <select className="input w-auto" value={severity} onChange={e => setSeverity(e.target.value)}>
        <option value="">All Severities</option>
        <option value="CRITICAL">Critical</option>
        <option value="HIGH">High</option>
        <option value="MEDIUM">Medium</option>
        <option value="LOW">Low</option>
        <option value="INFO">Info</option>
      </select>

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : events.length === 0 ? (
        <div className="card text-center py-12">
          <Shield size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No security events</h3>
          <p className="text-gray-400 mt-1">Events will appear when security rules trigger</p>
        </div>
      ) : (
        <div className="space-y-3">
          {events.map(event => (
            <div key={event.id} className="card flex items-start justify-between">
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <span className={`badge ${severityBadge[event.severity] || 'badge-gray'}`}>{event.severity}</span>
                  <h3 className="font-medium">{event.title}</h3>
                </div>
                {event.description && <p className="text-sm text-gray-400 mt-1">{event.description}</p>}
                <p className="text-xs text-gray-500 mt-2">{new Date(event.created_at).toLocaleString()} | Source: {event.source || 'System'}</p>
              </div>
              {!event.acknowledged && (
                <button onClick={() => handleAck(event.id)} className="btn-secondary btn-sm flex items-center gap-1">
                  <Check size={14} />Ack
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
