import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Navbar from '../components/Navbar'

let mockUser: { id: string; username: string } | null = null
let mockLogout = vi.fn()

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: mockUser, logout: mockLogout }),
}))

vi.mock('../components/ThemeToggle', () => ({ default: () => <span data-testid="theme" /> }))
vi.mock('../components/AccentPicker', () => ({ default: () => <span data-testid="accent" /> }))

function renderNavbar() {
  return render(
    <MemoryRouter>
      <Navbar />
    </MemoryRouter>
  )
}

describe('Navbar', () => {
  beforeEach(() => {
    mockUser = null
    mockLogout = vi.fn()
  })

  it('renders brand link', () => {
    renderNavbar()
    expect(screen.getByText('bebradio')).toHaveAttribute('href', '/')
  })

  it('shows Sign In and Register when logged out', () => {
    renderNavbar()
    expect(screen.getByText('Sign In')).toHaveAttribute('href', '/login')
    expect(screen.getByText('Register')).toHaveAttribute('href', '/register')
  })

  it('shows username and Logout when logged in', () => {
    mockUser = { id: '1', username: 'alice' }
    renderNavbar()
    expect(screen.getByText('alice')).toHaveAttribute('href', '/user/1')
    expect(screen.getByText('Logout')).toBeInTheDocument()
    expect(screen.queryByText('Sign In')).not.toBeInTheDocument()
  })

  it('calls logout on Logout click', () => {
    mockUser = { id: '1', username: 'alice' }
    renderNavbar()
    screen.getByText('Logout').click()
    expect(mockLogout).toHaveBeenCalled()
  })
})
