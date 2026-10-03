import { createContext, useContext, useState, useEffect, useCallback, useMemo } from 'react'
import { api, setAuthToken, setUnauthorizedHandler, setTokenRefreshedHandler, refreshAuth, ApiError } from '../lib/api'

interface User {
  id: string
  email?: string
  username: string
  role?: string
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

  const clearSession = useCallback(() => {
    localStorage.removeItem('token')
    setAuthToken(null)
    setToken(null)
    setUser(null)
  }, [])

  const fetchUser = useCallback(async (_tok: string) => {
    try {
      const data = await api.getMe()
      setUser(data.user)
    } catch (err) {
      // теряем сессию только при явном отказе (401/403): сбой сети или 5xx
      // не должны разлогинивать — иначе и был «спонтанный разлогин»
      if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
        clearSession()
      }
    }
    setLoading(false)
  }, [clearSession])

  useEffect(() => {
    setUnauthorizedHandler(clearSession)
    // ротированный access из refreshAuth → синк state (localStorage уже записан в api)
    setTokenRefreshedHandler((fresh) => {
      setAuthToken(fresh)
      setToken(fresh)
    })
    return () => {
      setUnauthorizedHandler(null)
      setTokenRefreshedHandler(null)
    }
  }, [clearSession])

  useEffect(() => {
    if (token) {
      setAuthToken(token)
      fetchUser(token)
      return
    }
    // localStorage пуст, но httpOnly-кука может жить → тихий restore
    let cancelled = false
    refreshAuth().then((fresh) => {
      if (cancelled) return
      if (fresh) {
        setAuthToken(fresh)
        setToken(fresh) // триггерит эффект выше → fetchUser
      } else {
        setLoading(false)
      }
    })
    return () => {
      cancelled = true
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
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Network error'
      return { success: false, error: message }
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
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Network error'
      return { success: false, error: message }
    }
  }, [])

  const logout = useCallback(() => {
    void api.logout() // best-effort: отзыв refresh-куки на сервере
    clearSession()
  }, [clearSession])

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
