import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
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

  it('shows username and Sign out when logged in', () => {
    mockUser = { id: '1', username: 'alice' }
    renderNavbar()
    fireEvent.click(screen.getByRole('button', { name: /open user navigation/i }))
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('Sign out')).toBeInTheDocument()
    expect(screen.queryByText('Sign In')).not.toBeInTheDocument()
  })

  it('calls logout on Sign out click', () => {
    mockUser = { id: '1', username: 'alice' }
    renderNavbar()
    fireEvent.click(screen.getByRole('button', { name: /open user navigation/i }))
    fireEvent.click(screen.getByText('Sign out'))
    expect(mockLogout).toHaveBeenCalled()
  })
})
