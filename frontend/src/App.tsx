import { lazy, Suspense, useEffect } from 'react'
import { Routes, Route } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import { ToastProvider } from './context/ToastContext'
import { applyAccent, getStoredAccent } from './lib/theme'
import Navbar from './components/Navbar'
import ProtectedRoute from './components/ProtectedRoute'

const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const Home = lazy(() => import('./pages/Home'))
const Room = lazy(() => import('./pages/Room'))
const Profile = lazy(() => import('./pages/Profile'))
const Settings = lazy(() => import('./pages/Settings'))

function NotFound() {
  return (
    <div className="loading">
      <div style={{ textAlign: 'center' }}>
        <h1 style={{ fontSize: 48, marginBottom: 8 }}>404</h1>
        <p style={{ marginBottom: 16 }}>Page not found</p>
        <a href="/" className="btn btn-secondary">Go Home</a>
      </div>
    </div>
  )
}

export default function App() {
  useEffect(() => {
    const saved = localStorage.getItem('theme')
    if (saved) {
      document.documentElement.setAttribute('data-theme', saved)
    } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
      document.documentElement.setAttribute('data-theme', 'dark')
    }
    applyAccent(getStoredAccent())
  }, [])

  return (
    <AuthProvider>
      <ToastProvider>
        <div className="app-root">
          <Navbar />
          <div className="app">
            <Suspense fallback={<div className="loading">Loading...</div>}>
              <Routes>
                <Route path="/login" element={<Login />} />
                <Route path="/register" element={<Register />} />
                <Route path="/" element={<Home />} />
                <Route path="/room/:roomId" element={<Room />} />
                <Route path="/user/:userId" element={<Profile />} />
                <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
                <Route path="*" element={<NotFound />} />
              </Routes>
            </Suspense>
          </div>
        </div>
      </ToastProvider>
    </AuthProvider>
  )
}
