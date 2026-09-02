const BASE_URL = '/api/v1'

class ApiClient {
  private token: string | null = null

  constructor() {
    this.token = localStorage.getItem('token')
  }

  setToken(token: string | null) {
    this.token = token
    if (token) {
      localStorage.setItem('token', token)
    } else {
      localStorage.removeItem('token')
    }
  }

  private async request<T>(method: string, path: string, body?: any): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const res = await fetch(`${BASE_URL}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    })

    if (res.status === 401) {
      this.setToken(null)
      window.location.href = '/login'
      throw new Error('Unauthorized')
    }

    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Request failed' }))
      throw new Error(err.error || err.message || 'Request failed')
    }

    return res.json()
  }

  get<T>(path: string) { return this.request<T>('GET', path) }
  post<T>(path: string, body?: any) { return this.request<T>('POST', path, body) }
  put<T>(path: string, body?: any) { return this.request<T>('PUT', path, body) }
  delete<T>(path: string) { return this.request<T>('DELETE', path) }
}

export const api = new ApiClient()

// Auth
export const authApi = {
  login: (email: string, password: string) => api.post<any>('/auth/login', { email, password }),
  register: (data: any) => api.post<any>('/auth/register', data),
  logout: () => api.post<any>('/auth/logout'),
  getMe: () => api.get<any>('/auth/me'),
  setup2FA: () => api.post<any>('/auth/2fa/setup'),
  enable2FA: (code: string) => api.post<any>('/auth/2fa/enable', { code }),
  disable2FA: (password: string, code: string) => api.post<any>('/auth/2fa/disable', { password, code }),
  verify2FA: (session_token: string, code: string) => api.post<any>('/auth/2fa/verify', { session_token, code }),
  useRecoveryCode: (session_token: string, code: string) => api.post<any>('/auth/2fa/recovery', { session_token, code }),
  changePassword: (current_password: string, new_password: string) => api.post<any>('/auth/password/change', { current_password, new_password }),
  getSessions: () => api.get<any[]>('/auth/sessions'),
  revokeSession: (id: string) => api.delete<any>(`/auth/sessions/${id}`),
}

// Dashboard
export const dashboardApi = {
  getStats: () => api.get<any>('/dashboard/stats'),
}

// Users
export const usersApi = {
  list: (params?: Record<string, string>) => {
    const q = params ? '?' + new URLSearchParams(params).toString() : ''
    return api.get<any>(`/users${q}`)
  },
  get: (id: string) => api.get<any>(`/users/${id}`),
  create: (data: any) => api.post<any>('/users', data),
  update: (id: string, data: any) => api.put<any>(`/users/${id}`, data),
  delete: (id: string) => api.delete<any>(`/users/${id}`),
  resetPassword: (id: string, password: string) => api.post<any>(`/users/${id}/reset-password`, { new_password: password }),
  reset2FA: (id: string) => api.post<any>(`/users/${id}/reset-2fa`),
  assignRole: (id: string, roleId: string) => api.post<any>(`/users/${id}/assign-role`, { role_id: roleId }),
}

// Roles
export const rolesApi = {
  list: () => api.get<any[]>('/roles'),
  create: (data: any) => api.post<any>('/roles', data),
  update: (id: string, data: any) => api.put<any>(`/roles/${id}`, data),
  delete: (id: string) => api.delete<any>(`/roles/${id}`),
  permissions: () => api.get<any[]>('/roles/permissions'),
}

// Agents
export const agentsApi = {
  list: () => api.get<any[]>('/agents'),
  get: (id: string) => api.get<any>(`/agents/${id}`),
  delete: (id: string) => api.delete<any>(`/agents/${id}`),
}

// Hosts
export const hostsApi = {
  list: (params?: Record<string, string>) => {
    const q = params ? '?' + new URLSearchParams(params).toString() : ''
    return api.get<any>(`/hosts${q}`)
  },
  get: (id: string) => api.get<any>(`/hosts/${id}`),
  getMetrics: (id: string, type?: string, hours?: number) => {
    let q = `?hours=${hours || 24}`
    if (type) q += `&type=${type}`
    return api.get<any[]>(`/hosts/${id}/metrics${q}`)
  },
  getLatestMetrics: (id: string) => api.get<any[]>(`/hosts/${id}/latest-metrics`),
  getInterfaces: (id: string) => api.get<any[]>(`/hosts/${id}/interfaces`),
}

// Monitors
export const monitorsApi = {
  list: () => api.get<any[]>('/monitors'),
  get: (id: string) => api.get<any>(`/monitors/${id}`),
  create: (data: any) => api.post<any>('/monitors', data),
  update: (id: string, data: any) => api.put<any>(`/monitors/${id}`, data),
  delete: (id: string) => api.delete<any>(`/monitors/${id}`),
  getChecks: (id: string) => api.get<any[]>(`/monitors/${id}/checks`),
}

// Scanner
export const scannerApi = {
  profiles: () => api.get<any[]>('/scanner/profiles'),
  scans: () => api.get<any[]>('/scanner/scans'),
  getScan: (id: string) => api.get<any>(`/scanner/scans/${id}`),
  startScan: (target: string, profileId?: string) => api.post<any>('/scanner/scan', { target, profile_id: profileId }),
  cancelScan: (id: string) => api.delete<any>(`/scanner/scans/${id}`),
  discoveredHosts: () => api.get<any[]>('/scanner/hosts'),
  hostPorts: (id: string) => api.get<any[]>(`/scanner/hosts/${id}/ports`),
  changes: () => api.get<any[]>('/scanner/changes'),
}

// Alerts
export const alertsApi = {
  list: (params?: Record<string, string>) => {
    const q = params ? '?' + new URLSearchParams(params).toString() : ''
    return api.get<any>(`/alerts${q}`)
  },
  stats: () => api.get<any>('/alerts/stats'),
  get: (id: string) => api.get<any>(`/alerts/${id}`),
  acknowledge: (id: string) => api.post<any>(`/alerts/${id}/acknowledge`),
  resolve: (id: string) => api.post<any>(`/alerts/${id}/resolve`),
}

// Incidents
export const incidentsApi = {
  list: (params?: Record<string, string>) => {
    const q = params ? '?' + new URLSearchParams(params).toString() : ''
    return api.get<any[]>(`/incidents${q}`)
  },
  create: (data: any) => api.post<any>('/incidents', data),
  get: (id: string) => api.get<any>(`/incidents/${id}`),
  update: (id: string, data: any) => api.put<any>(`/incidents/${id}`, data),
  addNote: (id: string, content: string) => api.post<any>(`/incidents/${id}/notes`, { content }),
}

// Security
export const securityApi = {
  events: (params?: Record<string, string>) => {
    const q = params ? '?' + new URLSearchParams(params).toString() : ''
    return api.get<any[]>(`/security/events${q}`)
  },
  acknowledgeEvent: (id: string) => api.post<any>(`/security/events/${id}/acknowledge`),
  rules: () => api.get<any[]>('/security/rules'),
  createRule: (data: any) => api.post<any>('/security/rules', data),
  fimEvents: () => api.get<any[]>('/security/fim'),
  createFIMRule: (data: any) => api.post<any>('/security/fim/rules', data),
  vulnerabilities: () => api.get<any[]>('/security/vulnerabilities'),
}

// Audit
export const auditApi = {
  list: (params?: Record<string, string>) => {
    const q = params ? '?' + new URLSearchParams(params).toString() : ''
    return api.get<any[]>(`/audit${q}`)
  },
}

// Settings
export const settingsApi = {
  list: () => api.get<any[]>('/settings'),
  update: (data: Record<string, any>) => api.put<any>('/settings', data),
}

// Notifications
export const notificationsApi = {
  channels: () => api.get<any[]>('/notifications/channels'),
  createChannel: (data: any) => api.post<any>('/notifications/channels', data),
  deliveries: () => api.get<any[]>('/notifications/deliveries'),
}

// Logs
export const logsApi = {
  list: (params?: Record<string, string>) => {
    const q = params ? '?' + new URLSearchParams(params).toString() : ''
    return api.get<any[]>(`/logs${q}`)
  },
  sources: () => api.get<any[]>('/logs/sources'),
}
