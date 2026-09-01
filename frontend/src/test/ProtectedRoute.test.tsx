import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import ProtectedRoute from '../components/ProtectedRoute'

let mockUser: { id: string; email: string; username: string } | null = null
let mockLoading = false

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: mockUser, loading: mockLoading }),
}))

describe('ProtectedRoute', () => {
  it('renders children when authenticated', () => {
    mockUser = { id: '1', email: 'a@b.com', username: 'Alice' }
    mockLoading = false
    render(
      <MemoryRouter>
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      </MemoryRouter>
    )
    expect(screen.getByText('Protected Content')).toBeInTheDocument()
  })

  it('redirects to login when not authenticated', () => {
    mockUser = null
    mockLoading = false
    render(
      <MemoryRouter initialEntries={['/protected']}>
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      </MemoryRouter>
    )
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
  })

  it('shows loading when loading', () => {
    mockUser = null
    mockLoading = true
    render(
      <MemoryRouter>
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      </MemoryRouter>
    )
    expect(screen.getByText('Loading...')).toBeInTheDocument()
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
  })
})
