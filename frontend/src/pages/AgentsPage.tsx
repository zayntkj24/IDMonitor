import { useState, useEffect } from 'react'
import { agentsApi } from '../lib/api'
import { Database, Wifi, WifiOff } from 'lucide-react'

export default function AgentsPage() {
  const [agents, setAgents] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => { agentsApi.list().then(setAgents).finally(() => setLoading(false)) }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Agents</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : agents.length === 0 ? (
        <div className="card text-center py-12">
          <Database size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No agents registered</h3>
          <p className="text-gray-400 mt-1">Install and run the IDmonitor agent to register</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {agents.map(agent => (
            <div key={agent.id} className="card">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-semibold">{agent.name}</h3>
                  <p className="text-sm text-gray-400">{agent.hostname || agent.ip_address || '-'}</p>
                </div>
                <span className={`badge ${agent.status === 'ONLINE' ? 'badge-green' : 'badge-red'}`}>
                  {agent.status === 'ONLINE' ? <Wifi size={12} className="mr-1" /> : <WifiOff size={12} className="mr-1" />}
                  {agent.status}
                </span>
              </div>
              <div className="mt-4 text-sm text-gray-400 space-y-1">
                <p>Version: {agent.version || '-'}</p>
                <p>OS: {agent.os || '-'} {agent.os_version || ''}</p>
                <p>Last heartbeat: {agent.last_heartbeat ? new Date(agent.last_heartbeat).toLocaleString() : 'Never'}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
