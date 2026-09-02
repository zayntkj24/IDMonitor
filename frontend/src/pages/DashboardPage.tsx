import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { dashboardApi } from '../lib/api'
import { Server, Activity, AlertTriangle, Shield, Globe, Wifi, WifiOff, Bug } from 'lucide-react'

export default function DashboardPage() {
  const [stats, setStats] = useState<any>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    dashboardApi.getStats().then(setStats).catch(() => {}).finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>

  const cards = [
    { label: 'Hosts', value: stats?.hosts?.total || 0, sub: `${stats?.hosts?.online || 0} online`, icon: Server, color: 'blue', link: '/hosts' },
    { label: 'Agents', value: stats?.agents?.total || 0, sub: `${stats?.agents?.online || 0} online`, icon: Wifi, color: 'green', link: '/admin/agents' },
    { label: 'Open Alerts', value: stats?.alerts?.open || 0, sub: 'Active', icon: AlertTriangle, color: 'red', link: '/alerts' },
    { label: 'Incidents', value: stats?.incidents?.open || 0, sub: 'Active', icon: Shield, color: 'yellow', link: '/incidents' },
    { label: 'Vulnerabilities', value: stats?.vulnerabilities?.open || 0, sub: 'Open', icon: Bug, color: 'purple', link: '/vulnerabilities' },
    { label: 'Network Scans', value: stats?.scans?.total || 0, sub: 'Total', icon: Globe, color: 'blue', link: '/scanner/history' },
  ]

  const colorMap: Record<string, string> = {
    blue: 'bg-blue-900/30 text-blue-400',
    green: 'bg-green-900/30 text-green-400',
    red: 'bg-red-900/30 text-red-400',
    yellow: 'bg-yellow-900/30 text-yellow-400',
    purple: 'bg-purple-900/30 text-purple-400',
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <p className="text-gray-400 mt-1">Overview of your infrastructure</p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {cards.map(card => {
          const Icon = card.icon
          return (
            <Link key={card.label} to={card.link} className="card hover:border-gray-700 transition-colors group">
              <div className="flex items-start justify-between">
                <div>
                  <p className="text-sm text-gray-400">{card.label}</p>
                  <p className="text-3xl font-bold mt-1">{card.value}</p>
                  <p className="text-sm text-gray-500 mt-1">{card.sub}</p>
                </div>
                <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${colorMap[card.color]}`}>
                  <Icon size={20} />
                </div>
              </div>
            </Link>
          )
        })}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="card">
          <h2 className="font-semibold mb-4">Quick Actions</h2>
          <div className="space-y-2">
            <Link to="/hosts" className="flex items-center gap-3 p-3 rounded-lg hover:bg-gray-800 transition-colors">
              <Server size={18} className="text-blue-400" />
              <span>View Hosts</span>
            </Link>
            <Link to="/scanner" className="flex items-center gap-3 p-3 rounded-lg hover:bg-gray-800 transition-colors">
              <Globe size={18} className="text-green-400" />
              <span>Network Scanner</span>
            </Link>
            <Link to="/alerts" className="flex items-center gap-3 p-3 rounded-lg hover:bg-gray-800 transition-colors">
              <AlertTriangle size={18} className="text-yellow-400" />
              <span>View Alerts</span>
            </Link>
            <Link to="/security" className="flex items-center gap-3 p-3 rounded-lg hover:bg-gray-800 transition-colors">
              <Shield size={18} className="text-purple-400" />
              <span>Security Center</span>
            </Link>
          </div>
        </div>

        <div className="card">
          <h2 className="font-semibold mb-4">System Status</h2>
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 rounded-lg bg-gray-800/50">
              <div className="flex items-center gap-3">
                <Wifi size={18} className="text-green-400" />
                <span>API Server</span>
              </div>
              <span className="badge badge-green">Running</span>
            </div>
            <div className="flex items-center justify-between p-3 rounded-lg bg-gray-800/50">
              <div className="flex items-center gap-3">
                <Server size={18} className="text-blue-400" />
                <span>Database</span>
              </div>
              <span className="badge badge-green">Connected</span>
            </div>
            <div className="flex items-center justify-between p-3 rounded-lg bg-gray-800/50">
              <div className="flex items-center gap-3">
                <Activity size={18} className="text-yellow-400" />
                <span>Workers</span>
              </div>
              <span className="badge badge-green">Active</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
