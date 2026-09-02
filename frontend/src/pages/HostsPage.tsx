import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { hostsApi } from '../lib/api'
import { Server, Wifi, WifiOff } from 'lucide-react'

export default function HostsPage() {
  const [hosts, setHosts] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)

  useEffect(() => {
    hostsApi.list().then(data => { setHosts(data.hosts || []); setTotal(data.total || 0) }).finally(() => setLoading(false))
  }, [])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Hosts</h1>
          <p className="text-gray-400 mt-1">{total} host{total !== 1 ? 's' : ''} monitored</p>
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : hosts.length === 0 ? (
        <div className="card text-center py-12">
          <Server size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No hosts yet</h3>
          <p className="text-gray-400 mt-1">Install the IDmonitor agent on your servers to start monitoring</p>
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="text-left text-sm text-gray-400 border-b border-gray-800">
                <th className="pb-3 font-medium">Host</th>
                <th className="pb-3 font-medium">IP Address</th>
                <th className="pb-3 font-medium">OS</th>
                <th className="pb-3 font-medium">Status</th>
                <th className="pb-3 font-medium">Agent</th>
              </tr>
            </thead>
            <tbody>
              {hosts.map(host => (
                <tr key={host.id} className="table-row">
                  <td className="py-3">
                    <Link to={`/hosts/${host.id}`} className="text-brand-400 hover:text-brand-300 font-medium">
                      {host.name}
                    </Link>
                    {host.hostname && <p className="text-xs text-gray-500">{host.hostname}</p>}
                  </td>
                  <td className="py-3 text-sm text-gray-300">{host.ip_address || '-'}</td>
                  <td className="py-3 text-sm text-gray-300">{host.os || '-'}</td>
                  <td className="py-3">
                    <span className={`badge ${host.status === 'ONLINE' ? 'badge-green' : host.status === 'OFFLINE' ? 'badge-red' : 'badge-gray'}`}>
                      {host.status === 'ONLINE' ? <Wifi size={12} className="mr-1" /> : <WifiOff size={12} className="mr-1" />}
                      {host.status}
                    </span>
                  </td>
                  <td className="py-3 text-sm text-gray-300">{host.agent_name || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
