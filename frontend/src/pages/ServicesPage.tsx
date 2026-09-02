import { useState, useEffect } from 'react'
import { hostsApi } from '../lib/api'
import { Activity } from 'lucide-react'

export default function ServicesPage() {
  const [services, setServices] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    hostsApi.list().then(async (data) => {
      const allServices: any[] = []
      for (const host of (data.hosts || [])) {
        const h = await hostsApi.get(host.id)
        if (h.services) h.services.forEach((s: any) => { s.host_name = host.name; allServices.push(s) })
      }
      setServices(allServices)
    }).finally(() => setLoading(false))
  }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Services</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : services.length === 0 ? (
        <div className="card text-center py-12">
          <Activity size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No services yet</h3>
          <p className="text-gray-400 mt-1">Services will appear once agents report data</p>
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">Service</th>
              <th className="pb-3 font-medium">Host</th>
              <th className="pb-3 font-medium">Status</th>
              <th className="pb-3 font-medium">CPU</th>
              <th className="pb-3 font-medium">Memory</th>
            </tr></thead>
            <tbody>
              {services.map(svc => (
                <tr key={svc.id} className="table-row">
                  <td className="py-3 font-medium">{svc.name}</td>
                  <td className="py-3 text-sm text-gray-300">{svc.host_name}</td>
                  <td className="py-3"><span className={`badge ${svc.status === 'RUNNING' ? 'badge-green' : 'badge-red'}`}>{svc.status}</span></td>
                  <td className="py-3 text-sm">{svc.cpu_usage != null ? `${svc.cpu_usage.toFixed(1)}%` : '-'}</td>
                  <td className="py-3 text-sm">{svc.memory_usage != null ? `${svc.memory_usage.toFixed(1)}%` : '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
