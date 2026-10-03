import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  api,
  refreshAuth,
  setAuthToken,
  setUnauthorizedHandler,
  setTokenRefreshedHandler,
  setRefreshForbidden,
} from '../lib/api'

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) } as Response
}

describe('api refresh flow', () => {
  beforeEach(() => {
    localStorage.clear()
    setAuthToken('expired')
    setUnauthorizedHandler(null)
    setTokenRefreshedHandler(null)
    setRefreshForbidden(false)
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    setAuthToken(null)
    setUnauthorizedHandler(null)
    setTokenRefreshedHandler(null)
    setRefreshForbidden(false)
  })

  it('retries request with new token after refresh on 401', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(jsonResponse(401, { error: 'Not authenticated' }))
      .mockResolvedValueOnce(jsonResponse(200, { token: 'fresh' }))
      .mockResolvedValueOnce(jsonResponse(200, { user: { id: '1', username: 'alice' } }))

    const data = await api.getMe()

    expect(data.user.username).toBe('alice')
    expect(localStorage.getItem('token')).toBe('fresh')
    const calls = vi.mocked(fetch).mock.calls
    expect(calls).toHaveLength(3)
    expect(calls[2][0]).toBe('/api/auth/me')
    expect((calls[2][1] as RequestInit).headers).toMatchObject({
      Authorization: 'Bearer fresh',
    })
  })

  it('shares a single refresh across parallel 401s', async () => {
    vi.mocked(fetch).mockImplementation((url) => {
      const u = String(url)
      if (u.includes('/api/auth/refresh')) {
        return Promise.resolve(jsonResponse(200, { token: 'fresh' }))
      }
      // /me всегда 401: после refresh повтор тоже 401 (второй refresh не нужен)
      return Promise.resolve(jsonResponse(401, { error: 'Not authenticated' }))
    })

    const results = await Promise.allSettled([api.getMe(), api.getMe(), api.getMe()])

    expect(results.every((r) => r.status === 'rejected')).toBe(true)
    const refreshCalls = vi.mocked(fetch).mock.calls.filter(([u]) =>
      String(u).includes('/api/auth/refresh'),
    )
    expect(refreshCalls).toHaveLength(1)
  })

  it('calls unauthorized handler when refresh fails', async () => {
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    vi.mocked(fetch)
      .mockResolvedValueOnce(jsonResponse(401, { error: 'Not authenticated' }))
      .mockResolvedValueOnce(jsonResponse(401, { error: 'Not authenticated' }))

    await expect(api.getMe()).rejects.toMatchObject({ status: 401 })
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('does not refresh on login 401', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(401, { error: 'Bad credentials' }))

    await expect(api.login('a@b.com', 'bad')).rejects.toMatchObject({ status: 401 })
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1)
  })

  it('refreshAuth persists token and notifies handler', async () => {
    const onRefreshed = vi.fn()
    setTokenRefreshedHandler(onRefreshed)
    vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { token: 'fresh' }))

    const token = await refreshAuth()

    expect(token).toBe('fresh')
    expect(localStorage.getItem('token')).toBe('fresh')
    expect(onRefreshed).toHaveBeenCalledWith('fresh')
  })

  it('visitRoom retries after refresh', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(jsonResponse(401, { error: 'Not authenticated' }))
      .mockResolvedValueOnce(jsonResponse(200, { token: 'fresh' }))
      .mockResolvedValueOnce(jsonResponse(200, { ok: true }))

    await api.visitRoom('room1')

    const calls = vi.mocked(fetch).mock.calls
    expect(calls).toHaveLength(3)
    expect(calls[2][0]).toBe('/api/rooms/room1/visit')
    expect((calls[2][1] as RequestInit).headers).toMatchObject({
      Authorization: 'Bearer fresh',
    })
  })

  it('disables refresh after logout', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(jsonResponse(200, { ok: true }))

    await api.logout()
    const token = await refreshAuth()

    expect(token).toBeNull()
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1) // только сам logout
  })
})
