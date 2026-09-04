import { render, screen, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AuthProvider, useAuth } from '../context/AuthContext'

function TestConsumer() {
  const { user, token, loading, login, register, logout, authHeaders } = useAuth()
  return (
    <div>
      <span data-testid="loading">{String(loading)}</span>
      <span data-testid="user">{user ? user.username : 'null'}</span>
      <span data-testid="token">{token || 'null'}</span>
      <button onClick={() => login('a@b.com', 'pass')}>login</button>
      <button onClick={() => register('a@b.com', 'alice', 'pass')}>register</button>
      <button onClick={logout}>logout</button>
      <button onClick={() => {
        const h = authHeaders()
        document.title = JSON.stringify(h)
      }}>headers</button>
    </div>
  )
}

function renderWithAuth() {
  return render(
    <AuthProvider>
      <TestConsumer />
    </AuthProvider>
  )
}

describe('AuthContext', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('starts in loading state with no token', async () => {
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))
    expect(screen.getByTestId('user').textContent).toBe('null')
  })

  it('fetches user on mount when token exists', async () => {
    localStorage.setItem('token', 'tok123')
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ user: { id: '1', email: 'a@b.com', username: 'alice' } }),
    } as Response)
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))
    expect(screen.getByTestId('user').textContent).toBe('alice')
    expect(fetch).toHaveBeenCalledWith('/api/auth/me', {
      headers: { Authorization: 'Bearer tok123' },
    })
  })

  it('clears token on invalid response', async () => {
    localStorage.setItem('token', 'bad')
    vi.mocked(fetch).mockResolvedValueOnce({ ok: false } as Response)
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))
    expect(screen.getByTestId('user').textContent).toBe('null')
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('login stores token and sets user', async () => {
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    vi.mocked(fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ token: 'newtok', user: { id: '1', email: 'a@b.com', username: 'bob' } }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ user: { id: '1', email: 'a@b.com', username: 'bob' } }),
      } as Response)

    screen.getByText('login').click()
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('bob'))
    expect(localStorage.getItem('token')).toBe('newtok')
  })

  it('login returns error on failure', async () => {
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ error: 'Bad credentials' }),
    } as Response)

    await act(async () => {
      screen.getByText('login').click()
      await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('null'))
    })
  })

  it('login returns network error on fetch failure', async () => {
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    vi.mocked(fetch).mockRejectedValueOnce(new Error('fail'))

    await act(async () => {
      screen.getByText('login').click()
      await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('null'))
    })
  })

  it('register stores token and sets user', async () => {
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    vi.mocked(fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ token: 'regtok', user: { id: '2', email: 'a@b.com', username: 'alice' } }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ user: { id: '2', email: 'a@b.com', username: 'alice' } }),
      } as Response)

    screen.getByText('register').click()
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('alice'))
    expect(localStorage.getItem('token')).toBe('regtok')
  })

  it('logout clears token and user', async () => {
    localStorage.setItem('token', 'tok')
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ user: { id: '1', email: 'a@b.com', username: 'alice' } }),
    } as Response)
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('alice'))

    await act(async () => {
      screen.getByText('logout').click()
    })
    expect(screen.getByTestId('user').textContent).toBe('null')
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('authHeaders returns bearer when token exists', async () => {
    localStorage.setItem('token', 'mytok')
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ user: { id: '1', email: 'a@b.com', username: 'alice' } }),
    } as Response)
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    screen.getByText('headers').click()
    await waitFor(() => expect(document.title).toBe('{"Authorization":"Bearer mytok"}'))
  })

  it('authHeaders returns empty when no token', async () => {
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    screen.getByText('headers').click()
    expect(document.title).toBe('{}')
  })
})
