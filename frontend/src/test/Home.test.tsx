import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Home from '../pages/Home'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ authHeaders: () => ({ Authorization: 'Bearer tok' }) }),
  AuthProvider: ({ children }: { children: React.ReactNode }) => children,
}))

const setRoomAccess = vi.fn()
vi.mock('../lib/roomAccess', () => ({
  setRoomAccess: (...args: unknown[]) => setRoomAccess(...args),
}))

function mockFetch(handler: (url: string, init?: RequestInit) => unknown) {
  globalThis.fetch = vi.fn((url: string, init?: RequestInit) =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve(handler(url, init)),
    }),
  ) as unknown as typeof fetch
}

describe('Home password rooms', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockFetch(() => [])
  })

  it('opens a create-room window with an optional password field', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    fireEvent.click(screen.getByText('Create Room'))
    expect(await screen.findByText('Create a room')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Room name')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Leave empty for an open room')).toBeInTheDocument()
  })

  it('sends the password when creating a room and stores the access token', async () => {
    mockFetch((url, init) => {
      if (url === '/api/rooms' && init?.method === 'POST') {
        return { id: 'ABC123', access: 'room-token', has_password: true }
      }
      return []
    })

    render(<MemoryRouter><Home /></MemoryRouter>)
    fireEvent.click(screen.getByText('Create Room'))

    fireEvent.change(await screen.findByPlaceholderText('Room name'), { target: { value: 'Party' } })
    fireEvent.change(screen.getByPlaceholderText('Leave empty for an open room'), { target: { value: 's3cret' } })
    fireEvent.click(screen.getByText('Create room'))

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/room/ABC123'))

    const body = JSON.parse((globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls
      .find((c) => c[0] === '/api/rooms' && c[1]?.method === 'POST')![1].body)
    expect(body).toEqual({ name: 'Party', password: 's3cret' })
    expect(setRoomAccess).toHaveBeenCalledWith('ABC123', 'room-token')
  })

  it('prompts for a password when joining a locked room, then joins', async () => {
    mockFetch((url, init) => {
      if (url === '/api/rooms/LOCKED' && !init) return { id: 'LOCKED', name: 'Secret', locked: true, has_password: true }
      if (url === '/api/rooms/LOCKED/join') return { access: 'granted' }
      return []
    })

    render(<MemoryRouter><Home /></MemoryRouter>)
    fireEvent.click(screen.getByText('Join by Code'))

    fireEvent.change(await screen.findByPlaceholderText('e.g. ABC123'), { target: { value: 'LOCKED' } })
    fireEvent.click(screen.getByText('Join room'))

    expect(await screen.findByText('Password required')).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('Room password'), { target: { value: 'open-sesame' } })
    fireEvent.click(screen.getByText('Enter room'))

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/room/LOCKED'))
    expect(setRoomAccess).toHaveBeenCalledWith('LOCKED', 'granted')
  })

  it('shows a lock icon for password-protected rooms in the list', async () => {
    mockFetch(() => [
      { id: 'AAA111', name: 'Open', user_count: 0, track_count: 0, is_playing: false, has_password: false },
      { id: 'BBB222', name: 'Closed', user_count: 0, track_count: 0, is_playing: false, has_password: true },
    ])

    render(<MemoryRouter><Home /></MemoryRouter>)
    const closed = await screen.findByText('Closed')
    expect(closed.closest('.home-card')?.textContent).toContain('\u{1F512}')
    expect((await screen.findByText('Open')).closest('.home-card')?.textContent).not.toContain('\u{1F512}')
  })
})
