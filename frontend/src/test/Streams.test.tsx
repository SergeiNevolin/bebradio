import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Streams from '../pages/Streams'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

let mockUser: { id: string; username: string; role: string } | null = null
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: mockUser }),
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

const streams = [
  {
    id: 'STR001',
    name: 'Nonstop Mashups',
    user_count: 4,
    track_count: 7,
    is_playing: true,
    features: { mashup_radio: { enabled: true, filter: { q: '', title: '', artist: '', owner: '' } } },
  },
]

describe('Streams page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUser = null
    mockFetch(() => [])
  })

  it('lists streams and hides creation for non-admins', async () => {
    mockFetch(() => streams)
    render(<MemoryRouter><Streams /></MemoryRouter>)
    expect(await screen.findByText('Nonstop Mashups')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Создать поток' })).not.toBeInTheDocument()
  })

  it('lets an admin create a stream', async () => {
    mockUser = { id: 'admin1', username: 'admin', role: 'admin' }
    mockFetch((url, init) => {
      if (url === '/api/streams/' && init?.method === 'POST') {
        return { ...streams[0], id: 'NEW001', name: 'Night Shift' }
      }
      return streams
    })
    render(<MemoryRouter><Streams /></MemoryRouter>)
    fireEvent.click(await screen.findByRole('button', { name: 'Создать поток' }))
    fireEvent.change(screen.getByPlaceholderText('Нонстоп-мешапы'), {
      target: { value: 'Night Shift' },
    })
    const createButtons = screen.getAllByRole('button', { name: 'Создать поток' })
    fireEvent.click(createButtons[createButtons.length - 1])
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/stream/NEW001'))
  })
})
