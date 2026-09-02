import { useState, useEffect } from 'react'
import { auditApi } from '../lib/api'
import { BookOpen } from 'lucide-react'

export default function AuditPage() {
  const [logs, setLogs] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [actionFilter, setActionFilter] = useState('')

  useEffect(() => {
    setLoading(true)
    const params: Record<string, string> = {}
    if (actionFilter) params.action = actionFilter
    auditApi.list(params).then(setLogs).finally(() => setLoading(false))
  }, [actionFilter])

  const actionColors: Record<string, string> = {
    LOGIN_SUCCESS: 'badge-green', LOGIN_FAILED: 'badge-red', LOGOUT: 'badge-gray',
    USER_CREATED: 'badge-blue', PASSWORD_CHANGED: 'badge-yellow', '2FA_ENABLED': 'badge-green',
    '2FA_DISABLED': 'badge-yellow', '2FA_FAILED': 'badge-red', ADMIN_RESET_USER_2FA: 'badge-red',
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Audit Logs</h1>

      <div className="flex gap-4">
        <select className="input w-auto" value={actionFilter} onChange={e => setActionFilter(e.target.value)}>
          <option value="">All Actions</option>
          <option value="LOGIN_SUCCESS">Login Success</option>
          <option value="LOGIN_FAILED">Login Failed</option>
          <option value="LOGOUT">Logout</option>
          <option value="USER_CREATED">User Created</option>
          <option value="PASSWORD_CHANGED">Password Changed</option>
          <option value="2FA_ENABLED">2FA Enabled</option>
          <option value="2FA_DISABLED">2FA Disabled</option>
          <option value="2FA_FAILED">2FA Failed</option>
          <option value="ADMIN_RESET_USER_2FA">Admin Reset 2FA</option>
        </select>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : logs.length === 0 ? (
        <div className="card text-center py-12">
          <BookOpen size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No audit logs</h3>
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">Timestamp</th>
              <th className="pb-3 font-medium">Action</th>
              <th className="pb-3 font-medium">Actor</th>
              <th className="pb-3 font-medium">Target</th>
              <th className="pb-3 font-medium">IP</th>
            </tr></thead>
            <tbody>
              {logs.map(log => (
                <tr key={log.id} className="table-row">
                  <td className="py-3 text-sm">{new Date(log.created_at).toLocaleString()}</td>
                  <td className="py-3"><span className={`badge ${actionColors[log.action] || 'badge-gray'}`}>{log.action}</span></td>
                  <td className="py-3 text-sm">{log.actor_email || log.actor_id || '-'}</td>
                  <td className="py-3 text-sm text-gray-400">{log.target_type ? `${log.target_type}${log.target_id ? ` (${log.target_id.substring(0, 8)}...)` : ''}` : '-'}</td>
                  <td className="py-3 text-sm text-gray-400">{log.ip_address || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
