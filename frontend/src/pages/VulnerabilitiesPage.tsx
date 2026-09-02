import { useState, useEffect } from 'react'
import { securityApi } from '../lib/api'
import { Bug } from 'lucide-react'

export default function VulnerabilitiesPage() {
  const [vulns, setVulns] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => { securityApi.vulnerabilities().then(setVulns).finally(() => setLoading(false)) }, [])

  const sevBadge: Record<string, string> = {
    CRITICAL: 'badge-red', HIGH: 'badge-red', MEDIUM: 'badge-yellow', LOW: 'badge-blue',
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Vulnerabilities</h1>
      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : vulns.length === 0 ? (
        <div className="card text-center py-12">
          <Bug size={48} className="mx-auto text-gray-600 mb-4" />
          <h3 className="text-lg font-medium">No vulnerabilities found</h3>
          <p className="text-gray-400 mt-1">Vulnerability data will appear from agent software inventory</p>
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full">
            <thead><tr className="text-left text-sm text-gray-400 border-b border-gray-800">
              <th className="pb-3 font-medium">CVE</th>
              <th className="pb-3 font-medium">Title</th>
              <th className="pb-3 font-medium">Severity</th>
              <th className="pb-3 font-medium">CVSS</th>
              <th className="pb-3 font-medium">Status</th>
            </tr></thead>
            <tbody>
              {vulns.map(v => (
                <tr key={v.id} className="table-row">
                  <td className="py-3 font-mono text-sm">{v.cve_id || '-'}</td>
                  <td className="py-3 font-medium">{v.title}</td>
                  <td className="py-3"><span className={`badge ${sevBadge[v.severity] || 'badge-gray'}`}>{v.severity}</span></td>
                  <td className="py-3 text-sm">{v.cvss_score?.toFixed(1) || '-'}</td>
                  <td className="py-3"><span className="badge badge-gray">{v.status}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
