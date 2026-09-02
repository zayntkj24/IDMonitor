import { useState, useEffect } from 'react'
import { notificationsApi } from '../lib/api'
import { MessageSquare } from 'lucide-react'

export default function NotificationsPage() {
  const [channels, setChannels] = useState<any[]>([])
  const [deliveries, setDeliveries] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([notificationsApi.channels(), notificationsApi.deliveries()])
      .then(([c, d]) => { setChannels(c || []); setDeliveries(d || []) })
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Notifications</h1>

      <div className="card">
        <h2 className="font-semibold mb-4">Channels</h2>
        {channels.length === 0 ? (
          <p className="text-gray-500 text-sm">No notification channels configured</p>
        ) : (
          <div className="space-y-2">
            {channels.map(c => (
              <div key={c.id} className="flex items-center justify-between p-3 bg-gray-800/50 rounded-lg">
                <div><span className="font-medium">{c.name}</span><span className="ml-2 badge badge-blue">{c.type}</span></div>
                <span className={`badge ${c.enabled ? 'badge-green' : 'badge-gray'}`}>{c.enabled ? 'Enabled' : 'Disabled'}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="card">
        <h2 className="font-semibold mb-4">Recent Deliveries</h2>
        {deliveries.length === 0 ? (
          <p className="text-gray-500 text-sm">No deliveries yet</p>
        ) : (
          <div className="space-y-2">
            {deliveries.map(d => (
              <div key={d.id} className="flex items-center justify-between p-3 bg-gray-800/50 rounded-lg">
                <div>
                  <span className="font-medium">{d.title || 'Notification'}</span>
                  <p className="text-sm text-gray-400">{d.message}</p>
                </div>
                <span className={`badge ${d.status === 'SENT' ? 'badge-green' : d.status === 'FAILED' ? 'badge-red' : 'badge-yellow'}`}>{d.status}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
