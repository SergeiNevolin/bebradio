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

function renderNavbar(onBurger = vi.fn()) {
  return {
    onBurger,
    ...render(
      <MemoryRouter>
        <Navbar onBurger={onBurger} />
      </MemoryRouter>
    ),
  }
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

  it('calls onBurger from the menu button', () => {
    const { onBurger } = renderNavbar()
    fireEvent.click(screen.getByRole('button', { name: 'Меню разделов' }))
    expect(onBurger).toHaveBeenCalledTimes(1)
  })

  it('shows Sign In and Register when logged out', () => {
    renderNavbar()
    expect(screen.getByText('Войти')).toHaveAttribute('href', '/login')
    expect(screen.getByText('Регистрация')).toHaveAttribute('href', '/register')
  })

  it('shows username and Sign out when logged in', () => {
    mockUser = { id: '1', username: 'alice' }
    renderNavbar()
    fireEvent.click(screen.getByRole('button', { name: /open user navigation/i }))
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('Выйти')).toBeInTheDocument()
    expect(screen.queryByText('Войти')).not.toBeInTheDocument()
  })

  it('calls logout on Sign out click', () => {
    mockUser = { id: '1', username: 'alice' }
    renderNavbar()
    fireEvent.click(screen.getByRole('button', { name: /open user navigation/i }))
    fireEvent.click(screen.getByText('Выйти'))
    expect(mockLogout).toHaveBeenCalled()
  })
})
