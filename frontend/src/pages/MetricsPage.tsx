import { useState, useEffect } from 'react'
import { hostsApi } from '../lib/api'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'

export default function MetricsPage() {
  const [hosts, setHosts] = useState<any[]>([])
  const [selectedHost, setSelectedHost] = useState<string>('')
  const [metrics, setMetrics] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    hostsApi.list().then(data => {
      const h = data.hosts || []
      setHosts(h)
      if (h.length > 0) setSelectedHost(h[0].id)
    }).finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (!selectedHost) return
    hostsApi.getMetrics(selectedHost, undefined, 24).then(setMetrics)
  }, [selectedHost])

  const cpuData = metrics.filter(m => m.name === 'cpu_usage').map(m => ({ time: new Date(m.recorded_at).toLocaleTimeString(), value: m.value })).reverse()
  const memData = metrics.filter(m => m.name === 'memory_usage').map(m => ({ time: new Date(m.recorded_at).toLocaleTimeString(), value: m.value })).reverse()
  const diskData = metrics.filter(m => m.name === 'disk_usage').map(m => ({ time: new Date(m.recorded_at).toLocaleTimeString(), value: m.value })).reverse()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Metrics</h1>
        <select className="input w-auto" value={selectedHost} onChange={e => setSelectedHost(e.target.value)}>
          {hosts.map(h => <option key={h.id} value={h.id}>{h.name}</option>)}
        </select>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : hosts.length === 0 ? (
        <div className="card text-center py-12"><p className="text-gray-400">No hosts available. Install an agent first.</p></div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <MetricChart title="CPU Usage" data={cpuData} color="#3b82f6" unit="%" />
          <MetricChart title="Memory Usage" data={memData} color="#a855f7" unit="%" />
          <MetricChart title="Disk Usage" data={diskData} color="#22c55e" unit="%" />
        </div>
      )}
    </div>
  )
}

function MetricChart({ title, data, color, unit }: { title: string; data: any[]; color: string; unit: string }) {
  return (
    <div className="card">
      <h3 className="font-medium mb-4">{title}</h3>
      {data.length === 0 ? (
        <p className="text-gray-500 text-sm text-center py-8">No data available yet</p>
      ) : (
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
            <XAxis dataKey="time" stroke="#6b7280" fontSize={12} />
            <YAxis stroke="#6b7280" fontSize={12} domain={[0, 100]} />
            <Tooltip contentStyle={{ backgroundColor: '#1f2937', border: '1px solid #374151', borderRadius: '8px' }} />
            <Line type="monotone" dataKey="value" stroke={color} strokeWidth={2} dot={false} />
          </LineChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
