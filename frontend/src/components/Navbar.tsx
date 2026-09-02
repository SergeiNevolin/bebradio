import { Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import ThemeToggle from './ThemeToggle'
import AccentPicker from './AccentPicker'

export default function Navbar() {
  const { user, logout } = useAuth()

  return (
    <nav className="navbar">
      <div className="navbar-inner">
        <Link to="/" className="navbar-brand">bebradio</Link>
        <div className="navbar-right">
          {user ? (
            <>
              <Link to={`/user/${user.id}`} className="navbar-user">{user.username}</Link>
              <button className="btn btn-secondary btn-sm" onClick={logout}>Logout</button>
            </>
          ) : (
            <>
              <Link to="/login" className="btn btn-secondary btn-sm">Sign In</Link>
              <Link to="/register" className="btn btn-sm">Register</Link>
            </>
          )}
          <AccentPicker />
          <ThemeToggle />
        </div>
      </div>
    </nav>
  )
}
