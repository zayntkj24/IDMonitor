import { useState, useEffect } from 'react'
import { usersApi } from '../lib/api'
import { Users, Plus, X, Trash2, Key, Shield } from 'lucide-react'
import { useAuth } from '../lib/AuthContext'

export default function UsersPage() {
  const { hasPermission } = useAuth()
  const [users, setUsers] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ email: '', username: '', password: '', display_name: '' })

  useEffect(() => { loadUsers() }, [])

  const loadUsers = () => { usersApi.list().then(data => setUsers(data.users || [])).finally(() => setLoading(false)) }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    await usersApi.create(form)
    setShowCreate(false)
    setForm({ email: '', username: '', password: '', display_name: '' })
    loadUsers()
  }

  const handleReset2FA = async (id: string) => {
    if (confirm('Reset 2FA for this user?')) {
      await usersApi.reset2FA(id)
      loadUsers()
    }
  }

  const handleResetPassword = async (id: string) => {
    const newPass = prompt('Enter new password (min 8 chars):')
    if (newPass && newPass.length >= 8) {
      await usersApi.resetPassword(id, newPass)
      alert('Password reset successfully')
    }
  }

  const handleDelete = async (id: string) => {
    if (confirm('Delete this user?')) {
      await usersApi.delete(id)
      loadUsers()
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Users</h1>
        {hasPermission('users.create') && (
          <button onClick={() => setShowCreate(true)} className="btn-primary btn-sm flex items-center gap-2"><Plus size={16} />Add User</button>
        )}
      </div>

      {showCreate && (
        <div className="card">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-semibold">Create User</h2>
            <button onClick={() => setShowCreate(false)} className="text-gray-400 hover:text-white"><X size={18} /></button>
          </div>
          <form onSubmit={handleCreate} className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div><label className="label">Email</label><input type="email" className="input" value={form.email} onChange={e => setForm({...form, email: e.target.value})} required /></div>
            <div><label className="label">Username</label><input className="input" value={form.username} onChange={e => setForm({...form, username: e.target.value})} required /></div>
            <div><label className="label">Password</label><input type="password" className="input" value={form.password} onChange={e => setForm({...form, password: e.target.value})} required minLength={8} /></div>
            <div><label className="label">Display Name</label><input className="input" value={form.display_name} onChange={e => setForm({...form, display_name: e.target.value})} /></div>
            <div className="md:col-span-2"><button type="submit" className="btn-primary">Create User</button></div>
          </form>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">User</th>
              <th className="pb-3 font-medium">Status</th>
              <th className="pb-3 font-medium">2FA</th>
              <th className="pb-3 font-medium">Last Login</th>
              <th className="pb-3 font-medium">Actions</th>
            </tr></thead>
            <tbody>
              {users.map(u => (
                <tr key={u.id} className="table-row">
                  <td className="py-3">
                    <p className="font-medium">{u.display_name || u.username}</p>
                    <p className="text-xs text-gray-500">{u.email}</p>
                  </td>
                  <td className="py-3"><span className={`badge ${u.status === 'ACTIVE' ? 'badge-green' : u.status === 'LOCKED' ? 'badge-red' : 'badge-gray'}`}>{u.status}</span></td>
                  <td className="py-3"><span className={`badge ${u.two_factor_enabled ? 'badge-green' : 'badge-gray'}`}>{u.two_factor_enabled ? 'Enabled' : 'Disabled'}</span></td>
                  <td className="py-3 text-sm text-gray-400">{u.last_login ? new Date(u.last_login).toLocaleString() : 'Never'}</td>
                  <td className="py-3">
                    <div className="flex gap-1">
                      <button onClick={() => handleResetPassword(u.id)} className="p-1.5 hover:bg-gray-700 rounded" title="Reset Password"><Key size={14} /></button>
                      <button onClick={() => handleReset2FA(u.id)} className="p-1.5 hover:bg-gray-700 rounded" title="Reset 2FA"><Shield size={14} /></button>
                      <button onClick={() => handleDelete(u.id)} className="p-1.5 hover:bg-gray-700 rounded text-red-400" title="Delete"><Trash2 size={14} /></button>
                    </div>
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
