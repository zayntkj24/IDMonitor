import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { api, authApi } from './api'

interface User {
  id: string
  email: string
  username: string
  display_name: string | null
  status: string
  two_factor_enabled: boolean
  roles: string[]
  permissions: string[]
}

interface AuthContextType {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<any>
  logout: () => void
  hasPermission: (perm: string) => boolean
  hasRole: (role: string) => boolean
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextType>({
  user: null,
  loading: true,
  login: async () => ({}),
  logout: () => {},
  hasPermission: () => false,
  hasRole: () => false,
  refreshUser: async () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  const refreshUser = async () => {
    try {
      const token = localStorage.getItem('token')
      if (!token) {
        setLoading(false)
        return
      }
      api.setToken(token)
      const data = await authApi.getMe()
      setUser(data)
    } catch {
      setUser(null)
      api.setToken(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { refreshUser() }, [])

  const login = async (email: string, password: string) => {
    const result = await authApi.login(email, password)
    if (result.token) {
      api.setToken(result.token)
      await refreshUser()
    }
    return result
  }

  const logout = () => {
    authApi.logout().catch(() => {})
    api.setToken(null)
    setUser(null)
  }

  const hasPermission = (perm: string) => {
    return user?.permissions?.includes(perm) ?? false
  }

  const hasRole = (role: string) => {
    return user?.roles?.includes(role) ?? false
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, hasPermission, hasRole, refreshUser }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => useContext(AuthContext)
