import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { hostsApi } from '../lib/api'
import { ArrowLeft, Cpu, HardDrive, Activity, Wifi, WifiOff } from 'lucide-react'

export default function HostDetailPage() {
  const { id } = useParams()
  const [host, setHost] = useState<any>(null)
  const [metrics, setMetrics] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id) return
    Promise.all([hostsApi.get(id), hostsApi.getLatestMetrics(id)])
      .then(([h, m]) => { setHost(h); setMetrics(m || []) })
      .finally(() => setLoading(false))
  }, [id])

  if (loading) return <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
  if (!host) return <div className="card">Host not found</div>

  const getMetric = (name: string) => metrics.find(m => m.name === name)

  const cpuMetric = getMetric('cpu_usage')
  const memMetric = getMetric('memory_usage')
  const diskMetric = getMetric('disk_usage')
  const load1m = getMetric('load_1m')

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/hosts" className="text-gray-400 hover:text-white"><ArrowLeft size={20} /></Link>
        <div>
          <h1 className="text-2xl font-bold">{host.name}</h1>
          <p className="text-gray-400">{host.hostname || host.ip_address || 'No address'}</p>
        </div>
        <span className={`badge ${host.status === 'ONLINE' ? 'badge-green' : 'badge-red'} ml-auto`}>
          {host.status === 'ONLINE' ? <Wifi size={12} className="mr-1" /> : <WifiOff size={12} className="mr-1" />}
          {host.status}
        </span>
      </div>

      {/* Metric Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard icon={Cpu} label="CPU" value={cpuMetric?.value} unit="%" color="blue" />
        <MetricCard icon={Activity} label="Memory" value={memMetric?.value} unit="%" color="purple" />
        <MetricCard icon={HardDrive} label="Disk" value={diskMetric?.value} unit="%" color="green" />
        <MetricCard icon={Activity} label="Load (1m)" value={load1m?.value} unit="" color="yellow" />
      </div>

      {/* Host Info */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="card">
          <h2 className="font-semibold mb-4">System Information</h2>
          <dl className="space-y-2 text-sm">
            <div className="flex justify-between"><dt className="text-gray-400">OS</dt><dd>{host.os || '-'}</dd></div>
            <div className="flex justify-between"><dt className="text-gray-400">OS Version</dt><dd>{host.os_version || '-'}</dd></div>
            <div className="flex justify-between"><dt className="text-gray-400">Kernel</dt><dd>{host.kernel || '-'}</dd></div>
            <div className="flex justify-between"><dt className="text-gray-400">Architecture</dt><dd>{host.architecture || '-'}</dd></div>
            <div className="flex justify-between"><dt className="text-gray-400">CPU Cores</dt><dd>{host.cpu_cores || '-'}</dd></div>
            <div className="flex justify-between"><dt className="text-gray-400">Total Memory</dt><dd>{host.total_memory ? `${(host.total_memory / 1073741824).toFixed(1)} GB` : '-'}</dd></div>
            <div className="flex justify-between"><dt className="text-gray-400">Agent</dt><dd>{host.agent_id ? 'Connected' : 'None'}</dd></div>
          </dl>
        </div>

        <div className="card">
          <h2 className="font-semibold mb-4">Services</h2>
          {host.services?.length > 0 ? (
            <div className="space-y-2">
              {host.services.map((svc: any) => (
                <div key={svc.id} className="flex items-center justify-between p-2 rounded-lg bg-gray-800/50">
                  <span className="text-sm">{svc.name}</span>
                  <span className={`badge ${svc.status === 'RUNNING' ? 'badge-green' : svc.status === 'STOPPED' ? 'badge-red' : 'badge-gray'}`}>
                    {svc.status}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-gray-500 text-sm">No services reported</p>
          )}
        </div>
      </div>
    </div>
  )
}

function MetricCard({ icon: Icon, label, value, unit, color }: { icon: any; label: string; value?: number; unit: string; color: string }) {
  const colorMap: Record<string, string> = {
    blue: 'bg-blue-900/30 text-blue-400',
    purple: 'bg-purple-900/30 text-purple-400',
    green: 'bg-green-900/30 text-green-400',
    yellow: 'bg-yellow-900/30 text-yellow-400',
  }
  return (
    <div className="card">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm text-gray-400">{label}</p>
          <p className="text-2xl font-bold mt-1">{value != null ? `${value.toFixed(1)}${unit}` : '-'}</p>
        </div>
        <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${colorMap[color]}`}>
          <Icon size={20} />
        </div>
      </div>
    </div>
  )
}
