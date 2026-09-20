import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Rooms from '../pages/Rooms'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

let mockUser: { id: string; username: string } | null = null
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: mockUser }),
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

const rooms = [
  { id: 'AAA111', name: 'Open', user_count: 3, track_count: 2, is_playing: true, has_password: false },
  { id: 'BBB222', name: 'Closed', user_count: 0, track_count: 0, is_playing: false, has_password: true },
]

describe('Rooms page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUser = null
    mockFetch(() => [])
  })

  it('renders the hero with room entry actions and stats', async () => {
    mockFetch(() => rooms)
    render(<MemoryRouter><Rooms /></MemoryRouter>)
    expect(await screen.findByText('Слушать музыку вместе с друзьями')).toBeInTheDocument()
    expect(screen.getByText('Create Room')).toBeInTheDocument()
    expect(screen.getByText('Join by Code')).toBeInTheDocument()
    expect(screen.getByText('rooms')).toBeInTheDocument()
    expect(screen.getByText('listening')).toBeInTheDocument()
  })

  it('renders the rooms browser with actions', async () => {
    mockFetch(() => rooms)
    render(<MemoryRouter><Rooms /></MemoryRouter>)
    expect(await screen.findByText('All rooms')).toBeInTheDocument()
    expect(screen.getByText('Open')).toBeInTheDocument()
    expect(screen.getByText('Create Room')).toBeInTheDocument()
    expect(screen.getByText('Join by Code')).toBeInTheDocument()
  })

  it('shows a lock icon for password-protected rooms', async () => {
    mockFetch(() => rooms)
    render(<MemoryRouter><Rooms /></MemoryRouter>)
    const closed = await screen.findByText('Closed')
    const card = closed.closest('[data-testid="room-card"]')
    expect(card?.querySelector('[title="С паролем"]')).not.toBeNull()
    expect(card?.querySelector('svg')).not.toBeNull()
  })

  it('opens a create-room window with an optional password field', async () => {
    render(<MemoryRouter><Rooms /></MemoryRouter>)
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

    render(<MemoryRouter><Rooms /></MemoryRouter>)
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

  it('prompts for a password when opening a locked room card, then joins', async () => {
    mockFetch((url) => {
      if (url === '/api/rooms') return rooms
      if (url === '/api/rooms/BBB222') {
        return { id: 'BBB222', name: 'Closed', locked: true, has_password: true }
      }
      if (url === '/api/rooms/BBB222/join') return { access: 'granted' }
      return []
    })

    render(<MemoryRouter><Rooms /></MemoryRouter>)
    fireEvent.click(await screen.findByText('Closed'))

    expect(await screen.findByText('Password required')).toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('Room password'), { target: { value: 'open-sesame' } })
    fireEvent.click(screen.getByText('Enter room'))

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/room/BBB222'))
    expect(setRoomAccess).toHaveBeenCalledWith('BBB222', 'granted')
  })

  it('shows recently played rooms for logged-in users', async () => {
    mockUser = { id: 'u1', username: 'alice' }
    mockFetch((url) => {
      if (url === '/api/rooms/recent') return [rooms[0]]
      if (url === '/api/rooms') return rooms
      return []
    })
    render(<MemoryRouter><Rooms /></MemoryRouter>)
    expect(await screen.findByText('Recently Played')).toBeInTheDocument()
  })
})
