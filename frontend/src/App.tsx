import { lazy, Suspense, useEffect, useState } from 'react'
import { Routes, Route, useLocation } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import { ToastProvider } from './context/ToastContext'
import { PlayerProvider, isOnKaraoke } from './context/PlayerContext'
import { useKaraoke } from './karaoke/store'
import { applyAccent, getStoredAccent } from './lib/theme'
import Navbar from './components/Navbar'
import Sidebar from './components/Sidebar'
import MobileTabBar from './components/MobileTabBar'
import ProtectedRoute from './components/ProtectedRoute'
import { GlobalPlayerBar, GlobalPlayerOverlays, PanelDock } from './components/player/GlobalPlayerUI'

const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const Home = lazy(() => import('./pages/Home'))
const Rooms = lazy(() => import('./pages/Rooms'))
const Mashups = lazy(() => import('./pages/Mashups'))
const Room = lazy(() => import('./pages/Room'))
const Profile = lazy(() => import('./pages/Profile'))
const Settings = lazy(() => import('./pages/Settings'))
const KaraokePage = lazy(() => import('./pages/KaraokePage'))

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

const SIDEBAR_RAIL_KEY = 'sidebar-rail'

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

  const location = useLocation()
  // Комнаты — иммерсивные со своим эфиром; караоке — на время пения:
  // сайдбар там — шторка поверх контента. Караоке живёт в общей строке,
  // его 100vw-брейкаут учитывает ширину сайдбара через --sidebar-w
  // (см. KaraokePage.module.css).
  const karaokeActive = useKaraoke((s) => s.soundActive)
  const overlayNav =
    location.pathname.startsWith('/room/') ||
    (isOnKaraoke(location.pathname) && karaokeActive)
  const [rail, setRail] = useState(() => {
    try {
      return localStorage.getItem(SIDEBAR_RAIL_KEY) === '1'
    } catch {
      return false
    }
  })
  const [drawer, setDrawer] = useState(false)
  // Ширина сайдбара для сетки (бургер-зона в шапке, брейкауты страниц).
  // На комнатах сайдбар — шторка, но зона бургера в шапке та же.
  const sidebarClass = rail ? 'sb-rail' : 'sb-full'
  useEffect(() => {
    setDrawer(false)
  }, [location.pathname])
  const toggleSidebar = () => {
    if (overlayNav) {
      setDrawer((v) => !v)
    } else {
      setRail((v) => {
        const next = !v
        try {
          localStorage.setItem(SIDEBAR_RAIL_KEY, next ? '1' : '0')
        } catch {
          /* ignore */
        }
        return next
      })
    }
  }

  return (
    <AuthProvider>
      <ToastProvider>
        <PlayerProvider>
        <div className={`app-root ${sidebarClass}`}>
          <Navbar onBurger={toggleSidebar} />
          <div className="app-body">
            <Sidebar
              collapsed={rail}
              overlay={overlayNav}
              open={drawer}
              onClose={() => setDrawer(false)}
              onNavigate={() => setDrawer(false)}
            />
            <div className="app">
            <Suspense fallback={<div className="loading">Loading...</div>}>
              <Routes>
                <Route path="/login" element={<Login />} />
                <Route path="/register" element={<Register />} />
                <Route path="/" element={<Home />} />
                <Route path="/rooms" element={<Rooms />} />
                <Route path="/mashup" element={<Mashups />} />
                <Route path="/room/:roomId" element={<Room />} />
                <Route path="/user/:userId" element={<Profile />} />
                <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
                <Route path="/karaoke" element={<KaraokePage />} />
                <Route path="*" element={<NotFound />} />
              </Routes>
            </Suspense>
          </div>
          </div>
        </div>
        <GlobalPlayerBar />
        <GlobalPlayerOverlays />
        <PanelDock />
        <MobileTabBar />
        </PlayerProvider>
      </ToastProvider>
    </AuthProvider>
  )
}
