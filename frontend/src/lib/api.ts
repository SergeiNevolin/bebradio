import { getRoomAccess } from './roomAccess'
import type { Track } from '../types'

let authToken: string | null = null
let refreshPromise: Promise<RefreshOutcome> | null = null
let unauthorizedHandler: (() => void) | null = null
let tokenRefreshedHandler: ((token: string) => void) | null = null
// после явного logout refresh запрещён: гонка «logout пришёл раньше refresh»
// иначе могла бы ротировать уже отозванную сессию обратно
let refreshForbidden = false

// для тестов / повторного входа
export function setRefreshForbidden(value: boolean) {
  refreshForbidden = value
}

export function setAuthToken(token: string | null) {
  authToken = token
}

// 401 без возможности обновиться → AuthContext разлогинивает
export function setUnauthorizedHandler(handler: (() => void) | null) {
  unauthorizedHandler = handler
}

// ротированный access → AuthContext синкает state
export function setTokenRefreshedHandler(handler: ((token: string) => void) | null) {
  tokenRefreshedHandler = handler
}

/**
 * Исход refresh: ok — новый access; rejected — сервер явно отказал (401:
 * чужой/протухший/отозванный RT — только тут разлогиниваем);
 * transient — сеть легла или бэкенд чихнул (5xx/503/обрыв): сессию храним,
 * следующий запрос попробует refresh снова.
 */
export type RefreshOutcome =
  | { ok: true; token: string }
  | { ok: false; rejected: boolean }

/** refresh упёрся не в 401, а в сеть/5xx: не разлогин, а «попробуйте снова». */
export class RefreshUnavailableError extends Error {
  constructor() {
    super('Сервер обновления недоступен — проверьте связь и попробуйте снова')
    this.name = 'RefreshUnavailableError'
  }
}

function authHeaders(): Record<string, string> {
  return authToken ? { Authorization: `Bearer ${authToken}` } : {}
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

// авторизационные мутации: на их 401 refresh не запускаем (иначе логин/логаут
// при неверном пароле ушёл бы в бесконечный refresh)
function isAuthMutation(url: string): boolean {
  const path = url.split('?')[0]
  return (
    path.startsWith('/api/auth/login') ||
    path.startsWith('/api/auth/register') ||
    path.startsWith('/api/auth/refresh') ||
    path.startsWith('/api/auth/logout')
  )
}

// exp access-токена (мс, Unix): нужен проактивному refresh в AuthContext.
// Не-JWT (моки в тестах) → null, тогда живём как раньше — только по факту 401.
export function getAccessExpiry(token: string): number | null {
  const part = token.split('.')[1]
  if (!part) return null
  try {
    const payload = JSON.parse(
      atob(part.replace(/-/g, '+').replace(/_/g, '/')),
    ) as { exp?: unknown }
    return typeof payload.exp === 'number' ? payload.exp * 1000 : null
  } catch {
    return null
  }
}

// single-flight: параллельные 401 делят один POST /api/auth/refresh
export function refreshAuth(): Promise<RefreshOutcome> {
  if (refreshForbidden) return Promise.resolve({ ok: false, rejected: true })
  if (!refreshPromise) {
    refreshPromise = (async (): Promise<RefreshOutcome> => {
      // snapshot for the quiet-restore case (no token in localStorage yet)
      const tokenAtStart = localStorage.getItem('token')
      try {
        const apply = async (res: Response): Promise<RefreshOutcome> => {
          if (!res.ok) {
            // 401 — сессия мертва; всё остальное — чих сети/бэкенда, не разлогин
            return { ok: false, rejected: res.status === 401 }
          }
          const data = (await res.json()) as { token?: string }
          if (!data.token) return { ok: false, rejected: true }
          // while the restore was in flight the user may have logged in or
          // registered — never overwrite that fresh session with the old token
          if (tokenAtStart === null && localStorage.getItem('token') !== null) {
            return { ok: false, rejected: true }
          }
          authToken = data.token
          localStorage.setItem('token', data.token)
          tokenRefreshedHandler?.(data.token)
          return { ok: true, token: data.token }
        }
        const first = await fetch('/api/auth/refresh', {
          method: 'POST',
          credentials: 'same-origin',
        })
        const outcome = await apply(first)
        // повтор при 401: гонка вкладок — соседняя уже ротировала куку,
        // повтор уходит с новым значением из cookie jar и спасает сессию
        if (outcome.ok || !outcome.rejected) return outcome
        await new Promise((r) => setTimeout(r, 300))
        try {
          const second = await fetch('/api/auth/refresh', {
            method: 'POST',
            credentials: 'same-origin',
          })
          return await apply(second)
        } catch {
          return { ok: false, rejected: false }
        }
      } catch {
        return { ok: false, rejected: false }
      } finally {
        refreshPromise = null
      }
    })()
  }
  return refreshPromise
}

// 401 → refresh (одна попытка) → повтор; rejected — unauthorizedHandler,
// transient — бросаем RefreshUnavailableError (сессия живёт)
async function withRefresh(fetcher: () => Promise<Response>, url: string): Promise<Response> {
  let res = await fetcher()
  if (res.status === 401 && !isAuthMutation(url)) {
    const outcome = await refreshAuth()
    if (outcome.ok) {
      res = await fetcher()
    } else if (outcome.rejected) {
      unauthorizedHandler?.()
    } else {
      throw new RefreshUnavailableError()
    }
  }
  return res
}

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const res = await withRefresh(
    () =>
      fetch(url, {
        ...options,
        headers: {
          'Content-Type': 'application/json',
          ...authHeaders(),
          ...options.headers,
        },
      }),
    url,
  )
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new ApiError(res.status, body.error || `Request failed (${res.status})`)
  }
  return res.json()
}

// не-JSON запросы с авторизацией (visit/delete/cover) — тот же 401→refresh→retry
async function authFetch(url: string, options: RequestInit, fallbackError: string): Promise<Response> {
  const res = await withRefresh(
    () => fetch(url, { ...options, headers: authHeaders() }),
    url,
  )
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string }
    throw new ApiError(res.status, body.error || fallbackError)
  }
  return res
}

// Auth
// Backend response types — shapes match exactly what the Go server returns.
// See: backend/internal/delivery/http/*_handler.go
// See: backend/internal/domain/entity/user.go (PublicProfile / ProfileWithEmail)

interface UserProfile {
  id: string
  username: string
  bio: string
  avatar_url: string
  created_at: number
}

interface UserProfileWithEmail extends UserProfile {
  email: string
  role: string
}

export const api = {
  // ── Auth ────────────────────────────────────────────────────────────
  // POST /api/auth/login → { token, user: ProfileWithEmail() }
  login: async (email: string, password: string) => {
    const data = await request<{ user: UserProfileWithEmail; token: string }>(
      '/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }
    )
    refreshForbidden = false
    return data
  },

  // POST /api/auth/register → { token, user: ProfileWithEmail() }
  register: async (email: string, username: string, password: string) => {
    const data = await request<{ user: UserProfileWithEmail; token: string }>(
      '/api/auth/register', { method: 'POST', body: JSON.stringify({ email, username, password }) }
    )
    refreshForbidden = false
    return data
  },

  // GET /api/auth/me → { user: ProfileWithEmail() }
  getMe: () => request<{ user: UserProfileWithEmail }>('/api/auth/me'),

  // POST /api/auth/logout → { ok } (отзыв refresh на сервере, best-effort)
  logout: async () => {
    refreshForbidden = true // с этого момента refresh не пытаемся
    try {
      await fetch('/api/auth/logout', { method: 'POST', credentials: 'same-origin' })
    } catch {
      // сессию всё равно чистим локально
    }
  },

  // ── Rooms ───────────────────────────────────────────────────────────
  // GET /api/rooms → []map  (raw array, not { rooms: [] }); чанки ?limit=&offset=
  getRooms: (limit = 50, offset = 0) =>
    request<Array<{ id: string; name: string; user_count: number; track_count: number; is_playing: boolean; has_password: boolean; auto_radio: boolean; is_stream: boolean }>>(`/api/rooms?limit=${limit}&offset=${offset}`),

  // GET /api/rooms/recent → []map
  getRecentRooms: () =>
    request<Array<{ id: string; name: string; user_count: number; track_count: number; is_playing: boolean; has_password: boolean; is_stream: boolean }>>('/api/rooms/recent'),

  // POST /api/rooms → ToDict() + { access }
  // is_stream marks an admin-run stream (admins only, enforced server-side).
  createRoom: (name: string, password?: string, opts?: { is_stream?: boolean }) =>
    request<{ id: string; name: string; access?: string }>(
      '/api/rooms', { method: 'POST', body: JSON.stringify({ name, password: password || undefined, is_stream: opts?.is_stream || undefined }) }
    ),

  // GET /api/rooms/:roomID → ToDict() or { id, name, locked, has_password }
  getRoomByCode: (code: string) =>
    request<{ id: string; name: string }>(`/api/rooms/${code}`),

  // POST /api/rooms/:roomID/join → { room, username, access }
  joinRoom: (roomId: string, password?: string) =>
    request<{ access: string }>(
      `/api/rooms/${roomId}/join`, { method: 'POST', body: JSON.stringify({ password }) }
    ),

  // GET /api/rooms/:roomID?access= → ToDict()
  getRoom: (roomId: string) => {
    const access = getRoomAccess(roomId)
    const query = access ? `?access=${encodeURIComponent(access)}` : ''
    return request<Record<string, unknown>>(`/api/rooms/${roomId}${query}`)
  },

  // POST /api/rooms/:roomID/visit → { ok: true }
  visitRoom: (roomId: string) =>
    authFetch(`/api/rooms/${roomId}/visit`, { method: 'POST' }, 'Failed to visit room').catch(() => {}),

  // POST /api/rooms/:roomID/queue?access= → track.ToDict()
  // A track is a track: pass either a YouTube { url } (+ optional { source }
  // hint for future providers) or a library { track_id } (mashup/upload
  // today, any library-backed source tomorrow). Old payloads keep working.
  addToQueue: (roomId: string, payload: { source?: string; url?: string; track_id?: string }) => {
    const access = getRoomAccess(roomId)
    const query = access ? `?access=${encodeURIComponent(access)}` : ''
    return request<Track>(
      `/api/rooms/${roomId}/queue${query}`, { method: 'POST', body: JSON.stringify(payload) }
    )
  },
  addTrack: (roomId: string, url: string) =>
    api.addToQueue(roomId, { url }),
  addTrackById: (roomId: string, track_id: string) =>
    api.addToQueue(roomId, { track_id }),

  // POST /api/rooms/:roomID/queue/:trackID/import → 202 Track.ToDict()
  // Admin only: saves a YouTube queue track into the bebradio library
  // (audio + cover). The row starts "processing", poll getTrack for readiness.
  importQueueTrack: (roomId: string, trackId: string) => {
    const access = getRoomAccess(roomId)
    const query = access ? `?access=${encodeURIComponent(access)}` : ''
    return request<Track>(
      `/api/rooms/${roomId}/queue/${trackId}/import${query}`, { method: 'POST' }
    )
  },

  // PATCH /api/rooms/:roomID → ToDict()
  updateRoom: (roomId: string, settings: Record<string, unknown>) =>
    request<Record<string, unknown>>(
      `/api/rooms/${roomId}`, { method: 'PATCH', body: JSON.stringify(settings) }
    ),

  // DELETE /api/rooms/:roomID → { ok: true }
  deleteRoom: (roomId: string) =>
    authFetch(`/api/rooms/${roomId}`, { method: 'DELETE' }, 'Failed to delete room').then(() => {}),

  // ── Search ──────────────────────────────────────────────────────────
  // POST /api/search → []map  (raw array, not { results: [] })
  searchTracks: (query: string) =>
    request<Array<{ id: string; title: string; artist: string; url: string; thumbnail: string; duration: number }>>(
      '/api/search', { method: 'POST', body: JSON.stringify({ query }) }
    ),

  // ── Users ───────────────────────────────────────────────────────────
  // GET /api/users/me → { user: ProfileWithEmail() }
  getMeProfile: () => request<{ user: UserProfileWithEmail }>('/api/users/me'),

  // PUT /api/users/me → { user: ProfileWithEmail() }
  updateMeProfile: (data: { username?: string; bio?: string; avatar_url?: string }) =>
    request<{ user: UserProfileWithEmail }>(
      '/api/users/me', { method: 'PUT', body: JSON.stringify(data) }
    ),

  // GET /api/users/:userID → { user: PublicProfile() }  (NO email field)
  getUser: (userId: string) =>
    request<{ user: UserProfile }>(`/api/users/${userId}`),

  // ── Tracks (uploads library) ──────────────────────────────────────────
  // GET /api/tracks?q=&sort=recent|top&limit=&offset= → []Track.ToDict()
  listTracks: (q = '', sort: 'recent' | 'top' = 'recent', limit = 30, offset = 0) => {
    const params = new URLSearchParams({
      sort,
      limit: String(limit),
      offset: String(offset),
    })
    if (q) params.set('q', q)
    return request<Track[]>(`/api/tracks/?${params.toString()}`)
  },

  // GET /api/tracks/mine → []Track.ToDict()  (auth)
  myTracks: () => request<Track[]>('/api/tracks/mine'),

  // GET /api/tracks/liked?limit=&offset= → []Track.ToDict()  (auth)
  likedTracks: (limit = 30, offset = 0) =>
    request<Track[]>(`/api/tracks/liked?limit=${limit}&offset=${offset}`),

  // GET /api/tracks/:id → Track.ToDict()
  getTrack: (id: string) => request<Track>(`/api/tracks/${id}`),

  // POST/DELETE /api/tracks/:id/like → { likes, liked }  (auth)
  likeTrack: (id: string) =>
    request<{ likes: number; liked: boolean }>(`/api/tracks/${id}/like`, { method: 'POST' }),
  unlikeTrack: (id: string) =>
    request<{ likes: number; liked: boolean }>(`/api/tracks/${id}/like`, { method: 'DELETE' }),

  // PUT /api/tracks/:id/cover (multipart) → { ok: true }  (auth + owner)
  uploadTrackCover: (id: string, cover: File) => {
    const form = new FormData()
    form.append('file', cover)
    return authFetch(`/api/tracks/${id}/cover`, { method: 'PUT', body: form }, 'Failed to upload cover').then(() => {})
  },

  // DELETE /api/tracks/:id → { ok: true }  (auth + owner)
  deleteTrack: (id: string) =>
    authFetch(`/api/tracks/${id}`, { method: 'DELETE' }, 'Failed to delete track').then(() => {}),

  // PATCH /api/tracks/:id → Track.ToDict()  (auth + owner)
  updateTrack: (id: string, data: { title: string; artist: string }) =>
    request<Track>(`/api/tracks/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  // POST /api/tracks (multipart) → 202 Track.ToDict()
  // XHR rather than fetch so the upload exposes progress; the browser sets the
  // multipart boundary, so we must NOT force a Content-Type here.
  uploadTrack: (
    data: { file: File; title: string; artist: string; cover?: File | null },
    onProgress?: (pct: number) => void,
  ) =>
    new Promise<Track>((resolve, reject) => {
      const form = new FormData()
      form.append('title', data.title)
      form.append('artist', data.artist)
      if (data.cover) form.append('cover', data.cover)
      form.append('file', data.file) // file LAST: the Go handler reads title/artist/cover before it
      // 401 → refresh (одна попытка) → повтор; rejected — разлогин через
      // общий handler, transient — RefreshUnavailableError (сессия живёт).
      // Без этого загрузка после простоя (> жизни access) всегда падала в 401.
      const send = (retried: boolean) => {
        const xhr = new XMLHttpRequest()
        xhr.open('POST', '/api/tracks/')
        const headers = authHeaders()
        if (headers.Authorization) xhr.setRequestHeader('Authorization', headers.Authorization)
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100))
        }
        xhr.onload = () => {
          if (xhr.status === 202 || xhr.status === 200) {
            try {
              resolve(JSON.parse(xhr.responseText) as Track)
            } catch {
              reject(new ApiError(xhr.status, 'Malformed server response'))
            }
            return
          }
          if (xhr.status === 401 && !retried) {
            void refreshAuth().then((outcome) => {
              if (outcome.ok) {
                send(true)
              } else {
                if (outcome.rejected) unauthorizedHandler?.()
                reject(
                  outcome.rejected
                    ? new ApiError(xhr.status, 'Not authenticated')
                    : new RefreshUnavailableError(),
                )
              }
            })
            return
          }
          let message = `Upload failed (${xhr.status})`
          try {
            message = (JSON.parse(xhr.responseText) as { error?: string }).error || message
          } catch {
            /* keep default */
          }
          reject(new ApiError(xhr.status, message))
        }
        xhr.onerror = () => reject(new ApiError(0, 'Network error during upload'))
        xhr.send(form)
      }
      send(false)
    }),

  // ── Admin ────────────────────────────────────────────────────────────
  adminSearchUsers: (q: string) =>
    request<Array<{ id: string; username: string; role: string }>>(
      `/api/admin/users?q=${encodeURIComponent(q)}`,
    ),

  adminPromote: (userId: string) =>
    request<{ ok: boolean }>('/api/admin/promote', {
      method: 'POST',
      body: JSON.stringify({ user_id: userId }),
    }),

  adminDemote: (userId: string) =>
    request<{ ok: boolean }>('/api/admin/demote', {
      method: 'POST',
      body: JSON.stringify({ user_id: userId }),
    }),
}

