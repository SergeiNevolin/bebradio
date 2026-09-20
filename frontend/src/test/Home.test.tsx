import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Home from '../pages/Home'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: null }),
  AuthProvider: ({ children }: { children: React.ReactNode }) => children,
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
  { id: 'AAA111', name: 'Party', user_count: 5, track_count: 3, is_playing: true, has_password: false, auto_radio: false },
  { id: 'BBB222', name: 'Chill', user_count: 0, track_count: 0, is_playing: false, has_password: true, auto_radio: false },
  { id: 'STN001', name: 'Nonstop Hits', user_count: 10, track_count: 4, is_playing: true, has_password: false, auto_radio: true },
]

const tracks = [
  { id: 't1', title: 'Hit One', artist: 'DJ A', likes: 42, thumbnail: '', status: 'ready' },
  { id: 't2', title: 'Hit Two', artist: 'DJ B', likes: 7, thumbnail: '', status: 'ready' },
]

function mockHomeApis(roomList = rooms) {
  mockFetch((url) => {
    if (url === '/api/rooms') return roomList
    if (url.startsWith('/api/tracks/')) return tracks
    return []
  })
}

describe('Home', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockHomeApis()
  })

  it('shelves autodj rooms as popular stations with a badge', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Потоки')).toBeInTheDocument()
    expect(screen.getByText('24/7')).toBeInTheDocument()
    expect(screen.getByText('Nonstop Hits')).toBeInTheDocument()
  })

  it('shows live rooms with a browse-all link', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Комнаты')).toBeInTheDocument()
    expect(screen.getAllByText('Party').length).toBeGreaterThanOrEqual(1)
  })

  it('hides the stations section when no room runs autodj', async () => {
    mockHomeApis(rooms.filter((r) => !r.auto_radio))
    render(<MemoryRouter><Home /></MemoryRouter>)
    await screen.findByText('Комнаты')
    expect(screen.queryByText('Потоки')).not.toBeInTheDocument()
  })

  it('shows live rooms with a browse-all link, stations excluded', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Комнаты')).toBeInTheDocument()
    expect(screen.getAllByText('Party').length).toBeGreaterThanOrEqual(1)
    // Idle rooms and stations belong to other shelves, not the live shelf.
    expect(screen.queryByText('Chill')).not.toBeInTheDocument()
    const browseLinks = screen.getAllByText('Все →')
    expect(browseLinks.length).toBeGreaterThanOrEqual(1)
    for (const link of browseLinks) {
      expect(link.closest('a')).toHaveAttribute('href', '/rooms')
    }
  })

  it('shows top mashups with a link to the mashups page', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Топ мэшапов')).toBeInTheDocument()
    expect(screen.getByText('Hit One')).toBeInTheDocument()
    const card = screen.getAllByTestId('top-track-card')[0]
    expect(card.textContent).toContain('DJ A')
    expect(card.textContent).toContain('42')
    expect(card.querySelector('svg')).not.toBeNull()
    const open = screen.getByText('Все мэшапы →')
    expect(open.closest('a')).toHaveAttribute('href', '/mashup')
  })

})
