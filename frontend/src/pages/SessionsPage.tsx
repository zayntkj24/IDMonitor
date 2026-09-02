import { useState, useEffect } from 'react'
import { authApi } from '../lib/api'
import { Eye, Trash2 } from 'lucide-react'

export default function SessionsPage() {
  const [sessions, setSessions] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => { authApi.getSessions().then(setSessions).finally(() => setLoading(false)) }, [])

  const handleRevoke = async (id: string) => {
    if (confirm('Revoke this session?')) {
      await authApi.revokeSession(id)
      setSessions(sessions.map(s => s.id === id ? { ...s, revoked_at: new Date().toISOString() } : s))
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Sessions</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">IP Address</th>
              <th className="pb-3 font-medium">User Agent</th>
              <th className="pb-3 font-medium">2FA</th>
              <th className="pb-3 font-medium">Status</th>
              <th className="pb-3 font-medium">Created</th>
              <th className="pb-3 font-medium">Actions</th>
            </tr></thead>
            <tbody>
              {sessions.map(s => (
                <tr key={s.id} className="table-row">
                  <td className="py-3 text-sm">{s.ip_address || '-'}</td>
                  <td className="py-3 text-sm text-gray-400 max-w-[200px] truncate">{s.user_agent || '-'}</td>
                  <td className="py-3"><span className={`badge ${s['2fa_verified'] ? 'badge-green' : 'badge-gray'}`}>{s['2fa_verified'] ? 'Verified' : 'N/A'}</span></td>
                  <td className="py-3"><span className={`badge ${s.is_active && !s.revoked_at ? 'badge-green' : 'badge-red'}`}>{s.is_active && !s.revoked_at ? 'Active' : 'Revoked'}{s.current ? ' (Current)' : ''}</span></td>
                  <td className="py-3 text-sm text-gray-400">{new Date(s.created_at).toLocaleString()}</td>
                  <td className="py-3">
                    {s.is_active && !s.revoked_at && !s.current && (
                      <button onClick={() => handleRevoke(s.id)} className="p-1.5 hover:bg-gray-700 rounded text-red-400" title="Revoke"><Trash2 size={14} /></button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
