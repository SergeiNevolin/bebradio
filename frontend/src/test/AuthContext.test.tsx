import { render, screen, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AuthProvider, useAuth } from '../context/AuthContext'
import { setRefreshForbidden } from '../lib/api'

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
    setRefreshForbidden(false)
    // неожиданные вызовы (bootstrap refresh и т.п.) — отказ, не молчание
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Unexpected fetch')))
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    setRefreshForbidden(false)
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
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer tok123' },
    })
  })

  it('clears token on 401 after failed refresh', async () => {
    localStorage.setItem('token', 'bad')
    vi.mocked(fetch)
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Not authenticated' }),
      } as Response) // /me
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Not authenticated' }),
      } as Response) // refresh #1
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: 'Not authenticated' }),
      } as Response) // refresh #2 (retry при гонке вкладок — тоже отказ)
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))
    expect(screen.getByTestId('user').textContent).toBe('null')
    expect(localStorage.getItem('token')).toBeNull()
    expect(vi.mocked(fetch).mock.calls.some(([u]) => String(u).includes('/api/auth/refresh'))).toBe(true)
  })

  it('keeps token on server error', async () => {
    localStorage.setItem('token', 'tok500')
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'Internal server error' }),
    } as Response)
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))
    expect(screen.getByTestId('user').textContent).toBe('null')
    expect(localStorage.getItem('token')).toBe('tok500')
  })

  it('restores session from refresh cookie when no local token', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ token: 'restored' }),
      } as Response) // bootstrap refresh
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ user: { id: '1', email: 'a@b.com', username: 'alice' } }),
      } as Response) // /me
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('alice'))
    expect(localStorage.getItem('token')).toBe('restored')
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

  it('logout clears token, user and revokes refresh', async () => {
    localStorage.setItem('token', 'tok')
    vi.mocked(fetch)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ user: { id: '1', email: 'a@b.com', username: 'alice' } }),
      } as Response) // /me
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ ok: true }),
      } as Response) // POST /api/auth/logout
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('alice'))

    await act(async () => {
      screen.getByText('logout').click()
    })
    expect(screen.getByTestId('user').textContent).toBe('null')
    expect(localStorage.getItem('token')).toBeNull()
    expect(vi.mocked(fetch).mock.calls.some(([u]) => String(u) === '/api/auth/logout')).toBe(true)
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

  // access живёт минуты — вкладка, провисевшая дольше, не должна требовать вход
  const fakeJwt = (expSec: number) => `h.${btoa(JSON.stringify({ exp: expSec }))}.s`
  const okUser = () => Promise.resolve({
    ok: true,
    json: () => Promise.resolve({ user: { id: '1', email: 'a@b.com', username: 'alice' } }),
  } as Response)

  it('proactively refreshes expired access on mount', async () => {
    const expired = fakeJwt(Math.floor(Date.now() / 1000) - 60)
    const fresh = fakeJwt(Math.floor(Date.now() / 1000) + 900)
    localStorage.setItem('token', expired)
    vi.mocked(fetch).mockImplementation((url) => {
      if (String(url).includes('/api/auth/refresh')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ token: fresh }) } as Response)
      }
      return okUser()
    })
    renderWithAuth()
    await waitFor(() => expect(localStorage.getItem('token')).toBe(fresh))
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('alice'))
  })

  it('clears session when proactive refresh is finally rejected', async () => {
    localStorage.setItem('token', fakeJwt(Math.floor(Date.now() / 1000) - 60))
    vi.mocked(fetch).mockImplementation((url) => {
      if (String(url).includes('/api/auth/refresh')) {
        return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({ error: 'nope' }) } as Response)
      }
      return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({ error: 'nope' }) } as Response)
    })
    renderWithAuth()
    await waitFor(() => expect(localStorage.getItem('token')).toBeNull())
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))
    expect(screen.getByTestId('user').textContent).toBe('null')
  })

  it('refreshes on visibilitychange when access is about to expire', async () => {
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
    const soon = fakeJwt(Math.floor(Date.now() / 1000) + 30)
    const fresh = fakeJwt(Math.floor(Date.now() / 1000) + 900)
    localStorage.setItem('token', soon)
    let refreshCalls = 0
    vi.mocked(fetch).mockImplementation((url) => {
      if (String(url).includes('/api/auth/refresh')) {
        refreshCalls += 1
        // первая (со старта) — обрыв сети: сессия живёт, токен прежний
        if (refreshCalls === 1) return Promise.reject(new TypeError('down'))
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ token: fresh }) } as Response)
      }
      return okUser()
    })
    renderWithAuth()
    await waitFor(() => expect(screen.getByTestId('user').textContent).toBe('alice'))
    await waitFor(() => expect(refreshCalls).toBeGreaterThanOrEqual(1))
    expect(localStorage.getItem('token')).toBe(soon)

    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'))
    })
    await waitFor(() => expect(localStorage.getItem('token')).toBe(fresh))
  })
})
