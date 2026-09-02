import { useState, useEffect } from 'react'
import { rolesApi } from '../lib/api'
import { UserCog } from 'lucide-react'

export default function RolesPage() {
  const [roles, setRoles] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => { rolesApi.list().then(setRoles).finally(() => setLoading(false)) }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Roles & Permissions</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {roles.map(role => (
            <div key={role.id} className="card">
              <div className="flex items-center gap-2 mb-2">
                <UserCog size={20} className="text-brand-400" />
                <h3 className="font-semibold">{role.name}</h3>
                {role.is_system && <span className="badge badge-blue">System</span>}
              </div>
              <p className="text-sm text-gray-400">{role.description || 'No description'}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
