import { useState } from 'react'
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/AuthContext'
import {
  LayoutDashboard, Server, Wifi, Activity, Shield, Search,
  Bell, AlertTriangle, FileText, Settings, Users, UserCog,
  Database, Globe, Lock, RefreshCw, ChevronDown, ChevronRight,
  LogOut, Menu, X, ScanLine, Eye, Zap, Bug, ListChecks,
  HardDrive, Network, Clock, MessageSquare, BookOpen
} from 'lucide-react'

interface NavItem {
  label: string
  path: string
  icon: any
  adminOnly?: boolean
  children?: { label: string; path: string; icon: any }[]
}

const navItems: NavItem[] = [
  { label: 'Overview', path: '/', icon: LayoutDashboard },
  {
    label: 'Infrastructure', path: '/hosts', icon: Server,
    children: [
      { label: 'Hosts', path: '/hosts', icon: Server },
      { label: 'Services', path: '/services', icon: Activity },
      { label: 'Processes', path: '/processes', icon: ListChecks },
    ]
  },
  {
    label: 'Network', path: '/scanner', icon: Network,
    children: [
      { label: 'Scanner', path: '/scanner', icon: ScanLine },
      { label: 'Discovered Hosts', path: '/scanner/discovered', icon: Globe },
      { label: 'Scan History', path: '/scanner/history', icon: Clock },
    ]
  },
  {
    label: 'Observability', path: '/metrics', icon: Activity,
    children: [
      { label: 'Metrics', path: '/metrics', icon: Activity },
      { label: 'Logs', path: '/logs', icon: FileText },
      { label: 'Uptime', path: '/uptime', icon: RefreshCw },
    ]
  },
  {
    label: 'Security', path: '/security', icon: Shield,
    children: [
      { label: 'Events', path: '/security', icon: Shield },
      { label: 'File Integrity', path: '/fim', icon: Lock },
      { label: 'Vulnerabilities', path: '/vulnerabilities', icon: Bug },
    ]
  },
  {
    label: 'Operations', path: '/alerts', icon: Bell,
    children: [
      { label: 'Alerts', path: '/alerts', icon: AlertTriangle },
      { label: 'Incidents', path: '/incidents', icon: AlertTriangle },
      { label: 'Notifications', path: '/notifications', icon: MessageSquare },
    ]
  },
  {
    label: 'Administration', path: '/admin/users', icon: Settings, adminOnly: true,
    children: [
      { label: 'Users', path: '/admin/users', icon: Users },
      { label: 'Roles', path: '/admin/roles', icon: UserCog },
      { label: 'Agents', path: '/admin/agents', icon: Database },
      { label: 'Sessions', path: '/admin/sessions', icon: Eye },
      { label: 'Audit Logs', path: '/admin/audit', icon: BookOpen },
      { label: 'Settings', path: '/admin/settings', icon: Settings },
    ]
  },
]

function SidebarItem({ item, collapsed }: { item: NavItem; collapsed: boolean }) {
  const location = useLocation()
  const [open, setOpen] = useState(
    item.children?.some(c => location.pathname === c.path || location.pathname.startsWith(c.path + '/'))
  )
  const isActive = item.children?.some(c => location.pathname === c.path) ?? false
  const Icon = item.icon

  if (item.children) {
    return (
      <div>
        <button
          onClick={() => setOpen(!open)}
          className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
            isActive ? 'bg-brand-600/20 text-brand-400' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'
          }`}
        >
          <Icon size={18} />
          {!collapsed && <span className="flex-1 text-left">{item.label}</span>}
          {!collapsed && (open ? <ChevronDown size={14} /> : <ChevronRight size={14} />)}
        </button>
        {open && !collapsed && (
          <div className="ml-4 mt-1 space-y-0.5">
            {item.children.map(child => {
              const ChildIcon = child.icon
              const active = location.pathname === child.path
              return (
                <Link
                  key={child.path}
                  to={child.path}
                  className={`flex items-center gap-3 px-3 py-1.5 rounded-lg text-sm transition-colors ${
                    active ? 'bg-brand-600/20 text-brand-400' : 'text-gray-500 hover:text-gray-300 hover:bg-gray-800'
                  }`}
                >
                  <ChildIcon size={16} />
                  <span>{child.label}</span>
                </Link>
              )
            })}
          </div>
        )}
      </div>
    )
  }

  return (
    <Link
      to={item.path}
      className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
        location.pathname === item.path ? 'bg-brand-600/20 text-brand-400' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'
      }`}
    >
      <Icon size={18} />
      {!collapsed && <span>{item.label}</span>}
    </Link>
  )
}

export default function DashboardLayout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [searchOpen, setSearchOpen] = useState(false)

  const handleLogout = () => { logout(); navigate('/login') }

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside className={`${sidebarOpen ? 'w-64' : 'w-16'} bg-gray-900 border-r border-gray-800 flex flex-col transition-all duration-200`}>
        <div className="p-4 border-b border-gray-800 flex items-center gap-3">
          <div className="w-8 h-8 bg-brand-600 rounded-lg flex items-center justify-center font-bold text-sm">
            ID
          </div>
          {sidebarOpen && <span className="font-bold text-lg">IDmonitor</span>}
        </div>

        <nav className="flex-1 p-3 space-y-1 overflow-y-auto">
          {navItems.filter(i => !i.adminOnly || user?.roles?.includes('ADMIN')).map(item => (
            <SidebarItem key={item.label} item={item} collapsed={!sidebarOpen} />
          ))}
        </nav>

        <div className="p-3 border-t border-gray-800">
          <div className="flex items-center gap-3 px-3 py-2">
            <div className="w-8 h-8 bg-gray-700 rounded-full flex items-center justify-center text-sm font-medium">
              {user?.email?.[0]?.toUpperCase() || 'U'}
            </div>
            {sidebarOpen && (
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate">{user?.display_name || user?.username}</div>
                <div className="text-xs text-gray-500 truncate">{user?.email}</div>
              </div>
            )}
          </div>
          <button onClick={handleLogout} className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-gray-400 hover:text-red-400 hover:bg-gray-800 transition-colors">
            <LogOut size={18} />
            {sidebarOpen && <span>Logout</span>}
          </button>
        </div>
      </aside>

      {/* Main */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top bar */}
        <header className="h-14 border-b border-gray-800 bg-gray-900/80 backdrop-blur-sm flex items-center px-4 gap-4">
          <button onClick={() => setSidebarOpen(!sidebarOpen)} className="text-gray-400 hover:text-white">
            {sidebarOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
          <button onClick={() => setSearchOpen(!searchOpen)} className="flex items-center gap-2 px-3 py-1.5 bg-gray-800 rounded-lg text-sm text-gray-400 hover:bg-gray-700 transition-colors">
            <Search size={16} />
            <span>Search...</span>
            <kbd className="ml-4 px-1.5 py-0.5 bg-gray-700 rounded text-xs">Ctrl+K</kbd>
          </button>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
