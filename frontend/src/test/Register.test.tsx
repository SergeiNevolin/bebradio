import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Register from '../pages/Register'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

const mockRegister = vi.fn()
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ register: mockRegister }),
  AuthProvider: ({ children }: { children: React.ReactNode }) => children,
}))

vi.mock('../components/ThemeToggle', () => ({
  default: () => <div data-testid="theme-toggle" />,
}))

describe('Register', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders email, username, and password inputs', () => {
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    )
    expect(screen.getByPlaceholderText('Email')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Username')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Password')).toBeInTheDocument()
    expect(screen.getByText('Register')).toBeInTheDocument()
  })

  it('calls register with correct params', async () => {
    mockRegister.mockResolvedValue({ success: true })
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'new@test.com' } })
    fireEvent.change(screen.getByPlaceholderText('Username'), { target: { value: 'Alice' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret' } })
    fireEvent.click(screen.getByText('Register'))

    expect(mockRegister).toHaveBeenCalledWith('new@test.com', 'Alice', 'secret')
  })

  it('navigates to home on success', async () => {
    mockRegister.mockResolvedValue({ success: true })
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'new@test.com' } })
    fireEvent.change(screen.getByPlaceholderText('Username'), { target: { value: 'Alice' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'secret' } })
    fireEvent.click(screen.getByText('Register'))

    await vi.waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/')
    })
  })

  it('shows error on failure', async () => {
    mockRegister.mockResolvedValue({ success: false, error: 'Email already registered' })
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    )

    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'dup@test.com' } })
    fireEvent.change(screen.getByPlaceholderText('Username'), { target: { value: 'Bob' } })
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'pass' } })
    fireEvent.click(screen.getByText('Register'))

    expect(await screen.findByText('Email already registered')).toBeInTheDocument()
  })

  it('links to login page', () => {
    render(
      <MemoryRouter>
        <Register />
      </MemoryRouter>
    )
    expect(screen.getByText('Sign in')).toHaveAttribute('href', '/login')
  })
})
