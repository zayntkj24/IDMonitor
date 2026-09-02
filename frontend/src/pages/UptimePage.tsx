import { useState, useEffect } from 'react'
import { monitorsApi } from '../lib/api'
import { RefreshCw, Plus, X } from 'lucide-react'

export default function UptimePage() {
  const [monitors, setMonitors] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ name: '', type: 'HTTP', url: '', host: '', port: '', interval_seconds: 60, timeout_seconds: 30 })
  const [creating, setCreating] = useState(false)

  useEffect(() => { loadMonitors() }, [])

  const loadMonitors = () => {
    monitorsApi.list().then(setMonitors).finally(() => setLoading(false))
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setCreating(true)
    try {
      await monitorsApi.create({ ...form, port: form.port ? parseInt(form.port) : null })
      setShowCreate(false)
      setForm({ name: '', type: 'HTTP', url: '', host: '', port: '', interval_seconds: 60, timeout_seconds: 30 })
      loadMonitors()
    } catch (err) { alert('Failed') } finally { setCreating(false) }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Uptime Monitoring</h1>
        <button onClick={() => setShowCreate(true)} className="btn-primary btn-sm flex items-center gap-2"><Plus size={16} />Add Monitor</button>
      </div>

      {showCreate && (
        <div className="card">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-semibold">Create Monitor</h2>
            <button onClick={() => setShowCreate(false)} className="text-gray-400 hover:text-white"><X size={18} /></button>
          </div>
          <form onSubmit={handleCreate} className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div><label className="label">Name</label><input className="input" value={form.name} onChange={e => setForm({...form, name: e.target.value})} required /></div>
            <div><label className="label">Type</label>
              <select className="input" value={form.type} onChange={e => setForm({...form, type: e.target.value})}>
                <option>HTTP</option><option>HTTPS</option><option>TCP</option><option>ICMP</option><option>DNS</option>
              </select>
            </div>
            <div><label className="label">URL / Host</label><input className="input" value={form.url || form.host} onChange={e => setForm({...form, url: e.target.value})} placeholder="https://example.com" /></div>
            <div><label className="label">Interval (seconds)</label><input type="number" className="input" value={form.interval_seconds} onChange={e => setForm({...form, interval_seconds: parseInt(e.target.value)})} /></div>
            <div className="md:col-span-2"><button type="submit" disabled={creating} className="btn-primary">{creating ? 'Creating...' : 'Create Monitor'}</button></div>
          </form>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : monitors.length === 0 ? (
        <div className="card text-center py-12">
          <RefreshCw size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No monitors configured</h3>
          <p className="text-gray-400 mt-1">Create your first uptime monitor</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {monitors.map(m => (
            <div key={m.id} className="card">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-medium">{m.name}</h3>
                  <p className="text-sm text-gray-400 mt-1">{m.url || m.host || '-'}</p>
                </div>
                <span className={`badge ${m.status === 'UP' ? 'badge-green' : m.status === 'DOWN' ? 'badge-red' : 'badge-gray'}`}>{m.status}</span>
              </div>
              <div className="mt-4 text-sm text-gray-400">
                <p>Uptime: <span className="text-white font-medium">{(m.uptime_percentage || 0).toFixed(2)}%</span></p>
                <p>Type: {m.type} | Interval: {m.interval_seconds}s</p>
                {m.last_check && <p>Last check: {new Date(m.last_check).toLocaleString()}</p>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
