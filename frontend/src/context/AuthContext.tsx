import { createContext, useContext, useState, useEffect, useCallback, useMemo } from 'react'
import { api, setAuthToken } from '../lib/api'

interface User {
  id: string
  email?: string
  username: string
}

interface AuthContextType {
  user: User | null
  token: string | null
  loading: boolean
  login: (email: string, password: string) => Promise<{ success: boolean; error?: string }>
  register: (email: string, username: string, password: string) => Promise<{ success: boolean; error?: string }>
  logout: () => void
  authHeaders: () => Record<string, string>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'))
  const [loading, setLoading] = useState(true)

  const fetchUser = useCallback(async (_tok: string) => {
    try {
      const data = await api.getMe()
      setUser(data.user)
    } catch {
      localStorage.removeItem('token')
      setToken(null)
      setUser(null)
    }
    setLoading(false)
  }, [])

  useEffect(() => {
    if (token) {
      setAuthToken(token)
      fetchUser(token)
    } else {
      setLoading(false)
    }
  }, [token, fetchUser])

  const login = useCallback(async (email: string, password: string) => {
    try {
      const data = await api.login(email, password)
      localStorage.setItem('token', data.token)
      setToken(data.token)
      setAuthToken(data.token)
      setUser(data.user)
      return { success: true }
    } catch {
      return { success: false, error: 'Network error' }
    }
  }, [])

  const register = useCallback(async (email: string, username: string, password: string) => {
    try {
      const data = await api.register(email, username, password)
      localStorage.setItem('token', data.token)
      setToken(data.token)
      setAuthToken(data.token)
      setUser(data.user)
      return { success: true }
    } catch {
      return { success: false, error: 'Network error' }
    }
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem('token')
    setToken(null)
    setAuthToken(null)
    setUser(null)
  }, [])

  const authHeaders = useCallback((): Record<string, string> => {
    if (!token) return {}
    return { Authorization: `Bearer ${token}` }
  }, [token])

  const value = useMemo(
    () => ({ user, token, loading, login, register, logout, authHeaders }),
    [user, token, loading, login, register, logout, authHeaders],
  )

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
