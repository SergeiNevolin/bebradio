import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Login from '../pages/Login'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

const mockLogin = vi.fn()
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ login: mockLogin }),
  AuthProvider: ({ children }: { children: React.ReactNode }) => children,
}))

vi.mock('../components/ThemeToggle', () => ({
  default: () => <div data-testid="theme-toggle" />,
}))

describe('Login', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders email and password inputs', () => {
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    )
    expect(screen.getByPlaceholderText('Email')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Password')).toBeInTheDocument()
    expect(screen.getByText('Sign In')).toBeInTheDocument()
  })

  it('calls login with email and password', async () => {
    mockLogin.mockResolvedValue({ success: true })
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'test@test.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'pass123' } })
    fireEvent.click(screen.getByText('Sign In'))

    expect(mockLogin).toHaveBeenCalledWith('test@test.com', 'pass123')
  })

  it('navigates to home on success', async () => {
    mockLogin.mockResolvedValue({ success: true })
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'test@test.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'pass123' } })
    fireEvent.click(screen.getByText('Sign In'))

    await vi.waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })
  })

  it('shows error on failure', async () => {
    mockLogin.mockResolvedValue({ success: false, error: 'Invalid credentials' })
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'bad@test.com' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'wrong' } })
    fireEvent.click(screen.getByText('Sign In'))

    expect(await screen.findByText('Invalid credentials')).toBeInTheDocument()
  })

  it('links to register page', () => {
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    )
    expect(screen.getByText('Register')).toHaveAttribute('href', '/register')
  })
})
