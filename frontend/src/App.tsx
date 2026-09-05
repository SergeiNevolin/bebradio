import { useEffect } from 'react'
import { Routes, Route } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import { ToastProvider } from './context/ToastContext'
import { applyAccent, getStoredAccent } from './lib/theme'
import Navbar from './components/Navbar'
import Home from './pages/Home'
import Room from './pages/Room'
import Login from './pages/Login'
import Register from './pages/Register'
import Profile from './pages/Profile'

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
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route path="/register" element={<Register />} />
              <Route path="/" element={<Home />} />
              <Route path="/room/:roomId" element={<Room />} />
              <Route path="/user/:userId" element={<Profile />} />
            </Routes>
          </div>
        </div>
      </ToastProvider>
    </AuthProvider>
  )
}
