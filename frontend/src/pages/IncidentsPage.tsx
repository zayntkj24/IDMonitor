import { useState, useEffect } from 'react'
import { incidentsApi } from '../lib/api'
import { AlertTriangle, Plus, X } from 'lucide-react'

export default function IncidentsPage() {
  const [incidents, setIncidents] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ title: '', description: '', severity: 'MEDIUM' })

  useEffect(() => { incidentsApi.list().then(setIncidents).finally(() => setLoading(false)) }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    await incidentsApi.create(form)
    setShowCreate(false)
    setForm({ title: '', description: '', severity: 'MEDIUM' })
    incidentsApi.list().then(setIncidents)
  }

  const sevBadge: Record<string, string> = {
    CRITICAL: 'badge-red', HIGH: 'badge-red', MEDIUM: 'badge-yellow', LOW: 'badge-blue',
  }

  const statusBadge: Record<string, string> = {
    OPEN: 'badge-red', INVESTIGATING: 'badge-yellow', CONTAINED: 'badge-blue', RESOLVED: 'badge-green', CLOSED: 'badge-gray',
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Incidents</h1>
        <button onClick={() => setShowCreate(true)} className="btn-primary btn-sm flex items-center gap-2"><Plus size={16} />New Incident</button>
      </div>

      {showCreate && (
        <div className="card">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-semibold">Create Incident</h2>
            <button onClick={() => setShowCreate(false)} className="text-gray-400 hover:text-white"><X size={18} /></button>
          </div>
          <form onSubmit={handleCreate} className="space-y-4">
            <div><label className="label">Title</label><input className="input" value={form.title} onChange={e => setForm({...form, title: e.target.value})} required /></div>
            <div><label className="label">Description</label><textarea className="input" rows={3} value={form.description} onChange={e => setForm({...form, description: e.target.value})} /></div>
            <div><label className="label">Severity</label>
              <select className="input" value={form.severity} onChange={e => setForm({...form, severity: e.target.value})}>
                <option>CRITICAL</option><option>HIGH</option><option>MEDIUM</option><option>LOW</option><option>INFO</option>
              </select>
            </div>
            <button type="submit" className="btn-primary">Create</button>
          </form>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : incidents.length === 0 ? (
        <div className="card text-center py-12">
          <AlertTriangle size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No incidents</h3>
        </div>
      ) : (
        <div className="space-y-3">
          {incidents.map(inc => (
            <div key={inc.id} className="card">
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className={`badge ${sevBadge[inc.severity]}`}>{inc.severity}</span>
                    <span className={`badge ${statusBadge[inc.status] || 'badge-gray'}`}>{inc.status}</span>
                    <h3 className="font-medium">{inc.title}</h3>
                  </div>
                  {inc.description && <p className="text-sm text-gray-400 mt-1">{inc.description}</p>}
                  <p className="text-xs text-gray-500 mt-2">Created {new Date(inc.created_at).toLocaleString()}</p>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
