import { useState, useEffect } from 'react'
import { scannerApi } from '../lib/api'
import { ScanLine, Play, X } from 'lucide-react'

export default function ScannerPage() {
  const [profiles, setProfiles] = useState<any[]>([])
  const [scans, setScans] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [target, setTarget] = useState('')
  const [profileId, setProfileId] = useState('')
  const [scanning, setScanning] = useState(false)
  const [result, setResult] = useState<any>(null)

  useEffect(() => {
    Promise.all([scannerApi.profiles(), scannerApi.scans()])
      .then(([p, s]) => { setProfiles(p || []); setScans(s || []) })
      .finally(() => setLoading(false))
  }, [])

  const handleScan = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!target) return
    setScanning(true)
    setResult(null)
    try {
      const job = await scannerApi.startScan(target, profileId || undefined)
      // Poll for completion
      const poll = async () => {
        const scan = await scannerApi.getScan(job.job_id)
        if (scan.status === 'COMPLETED' || scan.status === 'FAILED') {
          setResult(scan)
          setScanning(false)
          const s = await scannerApi.scans()
          setScans(s || [])
        } else {
          setTimeout(poll, 2000)
        }
      }
      setTimeout(poll, 2000)
    } catch (err) {
      alert('Failed to start scan')
      setScanning(false)
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Network Scanner</h1>

      <div className="card">
        <h2 className="font-semibold mb-4">New Scan</h2>
        <form onSubmit={handleScan} className="flex gap-4 items-end">
          <div className="flex-1">
            <label className="label">Target</label>
            <input className="input" value={target} onChange={e => setTarget(e.target.value)} placeholder="192.168.1.0/24 or hostname" required />
          </div>
          <div className="w-64">
            <label className="label">Profile</label>
            <select className="input" value={profileId} onChange={e => setProfileId(e.target.value)}>
              <option value="">Default (Common Ports)</option>
              {profiles.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          </div>
          <button type="submit" disabled={scanning || !target} className="btn-primary flex items-center gap-2">
            {scanning ? <><div className="animate-spin w-4 h-4 border-2 border-white border-t-transparent rounded-full" />Scanning...</> : <><Play size={16} />Scan</>}
          </button>
        </form>
      </div>

      {result && (
        <div className="card">
          <h2 className="font-semibold mb-4">Scan Result: {result.target}</h2>
          <div className="grid grid-cols-4 gap-4 mb-4">
            <div><p className="text-sm text-gray-400">Status</p><p className="font-medium">{result.status}</p></div>
            <div><p className="text-sm text-gray-400">Hosts Found</p><p className="font-medium">{result.hosts_discovered}</p></div>
            <div><p className="text-sm text-gray-400">Ports Found</p><p className="font-medium">{result.ports_discovered}</p></div>
            <div><p className="text-sm text-gray-400">Duration</p><p className="font-medium">{result.duration_ms ? `${(result.duration_ms / 1000).toFixed(1)}s` : '-'}</p></div>
          </div>
          {result.hosts?.length > 0 && (
            <div className="mt-4">
              <h3 className="font-medium mb-2">Discovered Hosts</h3>
              <div className="space-y-2">
                {result.hosts.map((h: any) => (
                  <div key={h.id} className="flex items-center justify-between p-3 bg-gray-800/50 rounded-lg">
                    <div>
                      <span className="font-medium">{h.ip_address}</span>
                      {h.hostname && <span className="text-gray-400 ml-2">({h.hostname})</span>}
                      {h.os_guess && <span className="text-gray-500 ml-2 text-sm">{h.os_guess}</span>}
                    </div>
                    <span className={`badge ${h.state === 'up' ? 'badge-green' : 'badge-gray'}`}>{h.state}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
          {result.changes?.length > 0 && (
            <div className="mt-4">
              <h3 className="font-medium mb-2">Changes Detected</h3>
              {result.changes.map((c: any) => (
                <div key={c.id} className="p-3 bg-yellow-900/20 border border-yellow-800 rounded-lg mb-2">
                  <span className="badge badge-yellow">{c.change_type}</span>
                  <span className="ml-2">{c.host_ip} {c.port ? `port ${c.port}/${c.protocol}` : ''}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <div className="card">
        <h2 className="font-semibold mb-4">Recent Scans</h2>
        {scans.length === 0 ? (
          <p className="text-gray-500 text-sm">No scans yet</p>
        ) : (
          <div className="space-y-2">
            {scans.slice(0, 10).map(s => (
              <div key={s.id} className="flex items-center justify-between p-3 bg-gray-800/50 rounded-lg">
                <div>
                  <span className="font-medium">{s.target}</span>
                  <span className="text-gray-500 ml-3 text-sm">{new Date(s.created_at).toLocaleString()}</span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-sm text-gray-400">{s.hosts_discovered} hosts / {s.ports_discovered} ports</span>
                  <span className={`badge ${s.status === 'COMPLETED' ? 'badge-green' : s.status === 'RUNNING' ? 'badge-blue' : s.status === 'FAILED' ? 'badge-red' : 'badge-gray'}`}>{s.status}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
