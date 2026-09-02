import { useState, useEffect } from 'react'
import { settingsApi } from '../lib/api'
import { Settings } from 'lucide-react'

export default function SettingsPage() {
  const [settings, setSettings] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => { settingsApi.list().then(setSettings).finally(() => setLoading(false)) }, [])

  const handleSave = async () => {
    setSaving(true)
    const updates: Record<string, any> = {}
    settings.forEach(s => { updates[s.key] = s.value })
    await settingsApi.update(updates)
    setSaving(false)
    alert('Settings saved')
  }

  const updateValue = (key: string, value: any) => {
    setSettings(settings.map(s => s.key === key ? { ...s, value } : s))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">System Settings</h1>
        <button onClick={handleSave} disabled={saving} className="btn-primary">{saving ? 'Saving...' : 'Save Settings'}</button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64"><div className="animate-spin w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full" /></div>
      ) : (
        <div className="space-y-4">
          {Array.from(new Set(settings.map(s => s.category))).map(category => (
            <div key={category} className="card">
              <h2 className="font-semibold mb-4 capitalize">{category}</h2>
              <div className="space-y-4">
                {settings.filter(s => s.category === category).map(setting => (
                  <div key={setting.key} className="flex items-center justify-between">
                    <div>
                      <label className="text-sm font-medium">{setting.key}</label>
                      {setting.description && <p className="text-xs text-gray-500">{setting.description}</p>}
                    </div>
                    <input
                      className="input w-48 text-right"
                      value={typeof setting.value === 'string' ? setting.value.replace(/"/g, '') : JSON.stringify(setting.value)}
                      onChange={e => updateValue(setting.key, e.target.value)}
                    />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
