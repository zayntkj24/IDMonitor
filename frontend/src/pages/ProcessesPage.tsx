import { useState, useEffect } from 'react'
import { hostsApi } from '../lib/api'
import { ListChecks } from 'lucide-react'

export default function ProcessesPage() {
  const [processes, setProcesses] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    hostsApi.list().then(async (data) => {
      const allProcs: any[] = []
      for (const host of (data.hosts || [])) {
        const h = await hostsApi.get(host.id)
        // Processes are in process_snapshots, we'll show services as proxy
        if (h.services) h.services.forEach((s: any) => { s.host_name = host.name; allProcs.push({ ...s, pid: s.pid || '-' }) })
      }
      setProcesses(allProcs)
    }).finally(() => setLoading(false))
  }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Processes</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : processes.length === 0 ? (
        <div className="card text-center py-12">
          <ListChecks size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No process data</h3>
          <p className="text-gray-400 mt-1">Process data will appear when agents report</p>
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">PID</th>
              <th className="pb-3 font-medium">Name</th>
              <th className="pb-3 font-medium">Host</th>
              <th className="pb-3 font-medium">CPU</th>
              <th className="pb-3 font-medium">Memory</th>
              <th className="pb-3 font-medium">Status</th>
            </tr></thead>
            <tbody>
              {processes.map((p, i) => (
                <tr key={i} className="table-row">
                  <td className="py-3 text-sm">{p.pid}</td>
                  <td className="py-3 font-medium">{p.name}</td>
                  <td className="py-3 text-sm text-gray-300">{p.host_name}</td>
                  <td className="py-3 text-sm">{p.cpu_usage != null ? `${p.cpu_usage.toFixed(1)}%` : '-'}</td>
                  <td className="py-3 text-sm">{p.memory_usage != null ? `${p.memory_usage.toFixed(1)}%` : '-'}</td>
                  <td className="py-3"><span className={`badge ${p.status === 'RUNNING' ? 'badge-green' : 'badge-gray'}`}>{p.status || 'Unknown'}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
