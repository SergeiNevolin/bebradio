import { useState, useRef, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import SearchBar from './SearchBar'
import styles from './Navbar.module.css'

export default function Navbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    if (menuOpen) document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [menuOpen])

  const handleLogout = () => {
    setMenuOpen(false)
    logout()
  }

  const handleNavigate = (path: string) => {
    setMenuOpen(false)
    navigate(path)
  }

  return (
    <nav className={styles.navbar}>
      <div className={styles.navbarInner}>
        <div className={styles.navbarLeft}>
          <Link to="/" className={styles.navbarBrand}>bebradio</Link>
          <Link to="/mashup" className={styles.navbarLink}>Mashups</Link>
        </div>
        <SearchBar />
        <div className={styles.navbarRight}>
          {user ? (
            <div className={styles.navbarUserMenu} ref={menuRef}>
              <button
                className={styles.navbarAvatarBtn}
                onClick={() => setMenuOpen(!menuOpen)}
                aria-label="Open user navigation menu"
              >
                <div className={styles.navbarAvatar}>
                  {user.username[0].toUpperCase()}
                </div>
              </button>

              {menuOpen && (
                <div className={styles.navbarDropdown}>
                  <div className={styles.navbarDropdownHeader}>
                    <div className={`${styles.navbarAvatar} ${styles.navbarAvatarSm}`}>
                      {user.username[0].toUpperCase()}
                    </div>
                    <div className={styles.navbarDropdownUser}>
                      <span className={styles.navbarDropdownUsername}>{user.username}</span>
                      <span className={styles.navbarDropdownEmail}>{user.email}</span>
                    </div>
                  </div>
                  <div className={styles.navbarDropdownDivider} />
                  <button className={styles.navbarDropdownItem} onClick={() => handleNavigate(`/user/${user.id}`)}>
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                      <path d="M8 8a3 3 0 100-6 3 3 0 000 6zm0 1c-3.3 0-6 1.8-6 4v1h12v-1c0-2.2-2.7-4-6-4z" />
                    </svg>
                    Your Profile
                  </button>
                  <button className={styles.navbarDropdownItem} onClick={() => handleNavigate('/settings')}>
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                      <path d="M8 4.754a3.246 3.246 0 100 6.492 3.246 3.246 0 000-6.492zM5.754 8a2.246 2.246 0 114.492 0 2.246 2.246 0 01-4.492 0z" />
                      <path d="M9.796 1.343c-.527-1.79-3.065-1.79-3.592 0l-.094.319a.873.873 0 01-1.255.52l-.292-.16c-1.64-.892-3.433.902-2.54 2.541l.159.292a.873.873 0 01-.52 1.255l-.319.094c-1.79.527-1.79 3.065 0 3.592l.319.094a.873.873 0 01.52 1.255l-.16.292c-.892 1.64.901 3.434 2.541 2.54l.292-.159a.873.873 0 011.255.52l.094.319c.527 1.79 3.065 1.79 3.592 0l.094-.319a.873.873 0 011.255-.52l.292.16c1.64.893 3.434-.902 2.54-2.541l-.159-.292a.873.873 0 01.52-1.255l.319-.094c1.79-.527 1.79-3.065 0-3.592l-.319-.094a.873.873 0 01-.52-1.255l.16-.292c.893-1.64-.902-3.433-2.541-2.54l-.292.159a.873.873 0 01-1.255-.52l-.094-.319zm-2.633.283c.246-.835 1.428-.835 1.674 0l.094.319a1.873 1.873 0 002.693 1.115l.291-.16c.764-.415 1.6.42 1.184 1.185l-.159.292a1.873 1.873 0 001.116 2.692l.318.094c.835.246.835 1.428 0 1.674l-.319.094a1.873 1.873 0 00-1.115 2.693l.16.291c.415.764-.42 1.6-1.185 1.184l-.291-.159a1.873 1.873 0 00-2.693 1.116l-.094.318c-.246.835-1.428.835-1.674 0l-.094-.319a1.873 1.873 0 00-2.692-1.115l-.292.16c-.764.415-1.6-.42-1.184-1.185l.159-.291A1.873 1.873 0 001.945 8.93l-.319-.094c-.835-.246-.835-1.428 0-1.674l.319-.094A1.873 1.873 0 003.06 4.377l-.16-.292c-.415-.764.42-1.6 1.185-1.184l.292.159a1.873 1.873 0 002.692-1.115l.094-.319z" />
                    </svg>
                    Settings
                  </button>
                  <div className={styles.navbarDropdownDivider} />
                  <button className={`${styles.navbarDropdownItem} ${styles.navbarDropdownItemDanger}`} onClick={handleLogout}>
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                      <path d="M2 3h5v1H2v8h5v1H1V2h6v1H2V3zm7.5 3.5l1-1 3 3-3 3-1-1 2-2H6v-1h6.5l-2-2z" />
                    </svg>
                    Sign out
                  </button>
                </div>
              )}
            </div>
          ) : (
            <>
              <Link to="/login" className="btn btn-secondary btn-sm">Sign In</Link>
              <Link to="/register" className="btn btn-secondary btn-sm">Register</Link>
            </>
          )}
        </div>
      </div>
    </nav>
  )
}
