import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { authApi, api } from '../lib/api'

export default function TwoFactorPage() {
  const [code, setCode] = useState('')
  const [useRecovery, setUseRecovery] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const token = localStorage.getItem('pending_2fa_token')
    if (!token) { navigate('/login'); return }

    try {
      let result
      if (useRecovery) {
        result = await authApi.useRecoveryCode(token, code)
      } else {
        result = await authApi.verify2FA(token, code)
      }
      localStorage.removeItem('pending_2fa_token')
      api.setToken(result.token)
      navigate('/')
    } catch (err: any) {
      setError(err.message || 'Invalid code')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-950">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="w-16 h-16 bg-brand-600 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <span className="text-2xl font-bold">ID</span>
          </div>
          <h1 className="text-2xl font-bold">Two-Factor Authentication</h1>
          <p className="text-gray-400 mt-1">
            {useRecovery ? 'Enter a recovery code' : 'Enter the 6-digit code from your authenticator app'}
          </p>
        </div>

        <div className="card">
          {error && (
            <div className="bg-red-900/30 border border-red-800 text-red-400 px-4 py-3 rounded-lg mb-4 text-sm">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="label">{useRecovery ? 'Recovery Code' : 'Authenticator Code'}</label>
              <input
                type="text"
                className="input text-center text-2xl tracking-widest"
                value={code}
                onChange={e => setCode(e.target.value)}
                placeholder={useRecovery ? 'XXXX-XXXX' : '000000'}
                maxLength={20}
                autoFocus
              />
            </div>
            <button type="submit" disabled={loading || !code} className="btn-primary w-full">
              {loading ? 'Verifying...' : 'Verify'}
            </button>
          </form>

          <button
            onClick={() => { setUseRecovery(!useRecovery); setCode(''); setError('') }}
            className="w-full mt-4 text-sm text-brand-400 hover:text-brand-300 text-center"
          >
            {useRecovery ? 'Use authenticator code' : 'Use a recovery code'}
          </button>
        </div>
      </div>
    </div>
  )
}
