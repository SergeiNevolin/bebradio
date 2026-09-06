import { getRoomAccess } from './roomAccess'
import type { Mashup } from '../types'

let authToken: string | null = null

export function setAuthToken(token: string | null) {
  authToken = token
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

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
      ...options.headers,
    },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new ApiError(res.status, body.error || `Request failed (${res.status})`)
  }
  return res.json()
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
}

export const api = {
  // ── Auth ────────────────────────────────────────────────────────────
  // POST /api/auth/login → { token, user: ProfileWithEmail() }
  login: (email: string, password: string) =>
    request<{ user: UserProfileWithEmail; token: string }>(
      '/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }
    ),

  // POST /api/auth/register → { token, user: ProfileWithEmail() }
  register: (email: string, username: string, password: string) =>
    request<{ user: UserProfileWithEmail; token: string }>(
      '/api/auth/register', { method: 'POST', body: JSON.stringify({ email, username, password }) }
    ),

  // GET /api/auth/me → { user: PublicProfile() }  (NO email field)
  getMe: () => request<{ user: UserProfile }>('/api/auth/me'),

  // ── Rooms ───────────────────────────────────────────────────────────
  // GET /api/rooms → []map  (raw array, not { rooms: [] })
  getRooms: () =>
    request<Array<{ id: string; name: string; user_count: number; track_count: number; is_playing: boolean; has_password: boolean }>>('/api/rooms'),

  // GET /api/rooms/recent → []map
  getRecentRooms: () =>
    request<Array<{ id: string; name: string; user_count: number; track_count: number; is_playing: boolean; has_password: boolean }>>('/api/rooms/recent'),

  // POST /api/rooms → ToDict() + { access }
  createRoom: (name: string, password?: string) =>
    request<{ id: string; name: string; access?: string }>(
      '/api/rooms', { method: 'POST', body: JSON.stringify({ name, password: password || undefined }) }
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
    fetch(`/api/rooms/${roomId}/visit`, {
      method: 'POST',
      headers: authHeaders(),
    }).catch(() => {}),

  // POST /api/rooms/:roomID/queue?access= → track.ToDict()
  addTrack: (roomId: string, url: string) => {
    const access = getRoomAccess(roomId)
    const query = access ? `?access=${encodeURIComponent(access)}` : ''
    return request<{ id: string }>(
      `/api/rooms/${roomId}/queue${query}`, { method: 'POST', body: JSON.stringify({ url }) }
    )
  },

  // PATCH /api/rooms/:roomID → ToDict()
  updateRoom: (roomId: string, settings: Record<string, unknown>) =>
    request<Record<string, unknown>>(
      `/api/rooms/${roomId}`, { method: 'PATCH', body: JSON.stringify(settings) }
    ),

  // DELETE /api/rooms/:roomID → { ok: true }
  deleteRoom: (roomId: string) =>
    fetch(`/api/rooms/${roomId}`, {
      method: 'DELETE',
      headers: authHeaders(),
    }).then(async (res) => {
      if (!res.ok) {
        const body = await res.json().catch(() => ({})) as { error?: string }
        throw new ApiError(res.status, body.error || 'Failed to delete room')
      }
    }),

  // ── Search ──────────────────────────────────────────────────────────
  // POST /api/search → []map  (raw array, not { results: [] })
  searchTracks: (query: string) =>
    request<Array<{ id: string; title: string; artist: string; url: string; thumbnail: string; duration: number }>>(
      '/api/search', { method: 'POST', body: JSON.stringify({ query }) }
    ),

  // ── Lyrics ──────────────────────────────────────────────────────────
  // GET /api/rooms/:roomID/lyrics?access=&lang= → { available, track_id, lang, auto, cues }
  getLyrics: (roomId: string) =>
    request<{ available: boolean; track_id: string; lang: string; auto: boolean; cues: Array<{ start: number; dur: number; text: string }> }>(
      `/api/rooms/${roomId}/lyrics`
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

  // ── Mashups ─────────────────────────────────────────────────────────
  // GET /api/mashups?q=&limit=&offset= → []Mashup.ToDict()
  listMashups: (q = '', limit = 30, offset = 0) => {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (q) params.set('q', q)
    return request<Mashup[]>(`/api/mashups/?${params.toString()}`)
  },

  // GET /api/mashups/mine → []Mashup.ToDict()  (auth)
  myMashups: () => request<Mashup[]>('/api/mashups/mine'),

  // GET /api/mashups/:id → Mashup.ToDict()
  getMashup: (id: string) => request<Mashup>(`/api/mashups/${id}`),

  // DELETE /api/mashups/:id → { ok: true }  (auth + owner)
  deleteMashup: (id: string) =>
    fetch(`/api/mashups/${id}`, { method: 'DELETE', headers: authHeaders() }).then(async (res) => {
      if (!res.ok) {
        const body = await res.json().catch(() => ({})) as { error?: string }
        throw new ApiError(res.status, body.error || 'Failed to delete mashup')
      }
    }),

  // POST /api/mashups (multipart) → 202 Mashup.ToDict()
  // XHR rather than fetch so the upload exposes progress; the browser sets the
  // multipart boundary, so we must NOT force a Content-Type here.
  uploadMashup: (
    data: { file: File; title: string; artist: string },
    onProgress?: (pct: number) => void,
  ) =>
    new Promise<Mashup>((resolve, reject) => {
      const form = new FormData()
      form.append('title', data.title)
      form.append('artist', data.artist)
      form.append('file', data.file) // file LAST: the Go handler reads title/artist before it
      const xhr = new XMLHttpRequest()
      xhr.open('POST', '/api/mashups/')
      const headers = authHeaders()
      if (headers.Authorization) xhr.setRequestHeader('Authorization', headers.Authorization)
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100))
      }
      xhr.onload = () => {
        if (xhr.status === 202 || xhr.status === 200) {
          try {
            resolve(JSON.parse(xhr.responseText) as Mashup)
          } catch {
            reject(new ApiError(xhr.status, 'Malformed server response'))
          }
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
    }),
}
