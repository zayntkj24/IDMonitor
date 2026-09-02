import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './lib/AuthContext'
import LoginPage from './pages/LoginPage'
import TwoFactorPage from './pages/TwoFactorPage'
import DashboardLayout from './components/DashboardLayout'
import DashboardPage from './pages/DashboardPage'
import HostsPage from './pages/HostsPage'
import HostDetailPage from './pages/HostDetailPage'
import ServicesPage from './pages/ServicesPage'
import ProcessesPage from './pages/ProcessesPage'
import UptimePage from './pages/UptimePage'
import MetricsPage from './pages/MetricsPage'
import LogsPage from './pages/LogsPage'
import SecurityPage from './pages/SecurityPage'
import FIMPage from './pages/FIMPage'
import VulnerabilitiesPage from './pages/VulnerabilitiesPage'
import ScannerPage from './pages/ScannerPage'
import ScanHistoryPage from './pages/ScanHistoryPage'
import DiscoveredHostsPage from './pages/DiscoveredHostsPage'
import AlertsPage from './pages/AlertsPage'
import IncidentsPage from './pages/IncidentsPage'
import NotificationsPage from './pages/NotificationsPage'
import UsersPage from './pages/UsersPage'
import RolesPage from './pages/RolesPage'
import AgentsPage from './pages/AgentsPage'
import SessionsPage from './pages/SessionsPage'
import AuditPage from './pages/AuditPage'
import SettingsPage from './pages/SettingsPage'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) return <div className="flex items-center justify-center h-screen"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
  if (!user) return <Navigate to="/login" />
  return <>{children}</>
}

function AppRoutes() {
  const { user } = useAuth()

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" /> : <LoginPage />} />
      <Route path="/2fa" element={<TwoFactorPage />} />
      <Route path="/" element={<PrivateRoute><DashboardLayout /></PrivateRoute>}>
        <Route index element={<DashboardPage />} />
        <Route path="hosts" element={<HostsPage />} />
        <Route path="hosts/:id" element={<HostDetailPage />} />
        <Route path="services" element={<ServicesPage />} />
        <Route path="processes" element={<ProcessesPage />} />
        <Route path="uptime" element={<UptimePage />} />
        <Route path="metrics" element={<MetricsPage />} />
        <Route path="logs" element={<LogsPage />} />
        <Route path="security" element={<SecurityPage />} />
        <Route path="fim" element={<FIMPage />} />
        <Route path="vulnerabilities" element={<VulnerabilitiesPage />} />
        <Route path="scanner" element={<ScannerPage />} />
        <Route path="scanner/history" element={<ScanHistoryPage />} />
        <Route path="scanner/discovered" element={<DiscoveredHostsPage />} />
        <Route path="alerts" element={<AlertsPage />} />
        <Route path="incidents" element={<IncidentsPage />} />
        <Route path="notifications" element={<NotificationsPage />} />
        <Route path="admin/users" element={<UsersPage />} />
        <Route path="admin/roles" element={<RolesPage />} />
        <Route path="admin/agents" element={<AgentsPage />} />
        <Route path="admin/sessions" element={<SessionsPage />} />
        <Route path="admin/audit" element={<AuditPage />} />
        <Route path="admin/settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  )
}
