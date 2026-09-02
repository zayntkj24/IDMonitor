import { useState, useEffect } from 'react'
import { scannerApi } from '../lib/api'
import { Globe } from 'lucide-react'

export default function DiscoveredHostsPage() {
  const [hosts, setHosts] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedHost, setSelectedHost] = useState<any>(null)
  const [ports, setPorts] = useState<any[]>([])

  useEffect(() => { scannerApi.discoveredHosts().then(setHosts).finally(() => setLoading(false)) }, [])

  const viewPorts = async (host: any) => {
    setSelectedHost(host)
    const p = await scannerApi.hostPorts(host.id)
    setPorts(p || [])
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Discovered Hosts</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : hosts.length === 0 ? (
        <div className="card text-center py-12">
          <Globe size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No discovered hosts</h3>
          <p className="text-gray-400 mt-1">Run a network scan to discover hosts</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 card">
            <table className="w-full">
              <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
                <th className="pb-3 font-medium">IP Address</th>
                <th className="pb-3 font-medium">Hostname</th>
                <th className="pb-3 font-medium">OS</th>
                <th className="pb-3 font-medium">State</th>
                <th className="pb-3 font-medium"></th>
              </tr></thead>
              <tbody>
                {hosts.map(h => (
                  <tr key={h.id} className="table-row cursor-pointer" onClick={() => viewPorts(h)}>
                    <td className="py-3 font-mono">{h.ip_address}</td>
                    <td className="py-3 text-sm">{h.hostname || '-'}</td>
                    <td className="py-3 text-sm text-gray-400">{h.os_guess || '-'}</td>
                    <td className="py-3"><span className={`badge ${h.state === 'up' ? 'badge-green' : 'badge-gray'}`}>{h.state}</span></td>
                    <td className="py-3"><button className="text-brand-400 text-sm hover:underline">Ports →</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="card">
            <h2 className="font-semibold mb-4">Ports {selectedHost ? `(${selectedHost.ip_address})` : ''}</h2>
            {!selectedHost ? (
              <p className="text-gray-500 text-sm">Click a host to view ports</p>
            ) : ports.length === 0 ? (
              <p className="text-gray-500 text-sm">No ports discovered</p>
            ) : (
              <div className="space-y-2">
                {ports.map(p => (
                  <div key={p.id} className="flex items-center justify-between p-2 bg-gray-800/50 rounded-lg text-sm">
                    <div>
                      <span className="font-mono font-medium">{p.port}/{p.protocol}</span>
                      {p.service && <span className="ml-2 text-gray-400">{p.service}</span>}
                    </div>
                    <span className={`badge ${p.state === 'open' ? 'badge-green' : 'badge-gray'}`}>{p.state}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
