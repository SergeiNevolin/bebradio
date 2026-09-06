import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Mashups from '../pages/Mashups'
import type { Mashup } from '../types'

const { listMashups, myMashups, getMashup, deleteMashup } = vi.hoisted(() => ({
  listMashups: vi.fn(),
  myMashups: vi.fn(),
  getMashup: vi.fn(),
  deleteMashup: vi.fn(),
}))
vi.mock('../lib/api', () => ({
  api: { listMashups, myMashups, getMashup, deleteMashup },
}))

let mockUser: { id: string; username: string } | null = null
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: mockUser }),
}))

const showToast = vi.fn()
vi.mock('../context/ToastContext', () => ({ useToast: () => ({ showToast }) }))

function mashup(over: Partial<Mashup> = {}): Mashup {
  return {
    id: 'm1',
    owner_id: 'owner1',
    title: 'Alpha Bootleg',
    artist: 'DJ A',
    duration: 180,
    size_bytes: 0,
    status: 'ready',
    has_cover: false,
    plays: 0,
    created_at: '2026-01-01T00:00:00Z',
    stream_url: '/api/mashups/media/aaa',
    ...over,
  }
}

function renderPage() {
  return render(<MemoryRouter><Mashups /></MemoryRouter>)
}

describe('Mashups page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUser = null
    listMashups.mockResolvedValue([mashup(), mashup({ id: 'm2', title: 'Beta Bootleg' })])
    myMashups.mockResolvedValue([mashup({ id: 'm9', title: 'My Only Mix' })])
  })

  it('renders the header and the loaded mashups', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: 'Mashups' })).toBeInTheDocument()
    expect(await screen.findAllByText('Alpha Bootleg')).not.toHaveLength(0)
    expect(screen.getAllByText('Beta Bootleg').length).toBeGreaterThan(0)
  })

  it('debounces the search box into a query request', async () => {
    renderPage()
    await screen.findAllByText('Alpha Bootleg')

    fireEvent.change(screen.getByLabelText('Search mashups'), { target: { value: 'beta' } })

    await waitFor(() => expect(listMashups).toHaveBeenLastCalledWith('beta'))
  })

  it('hides upload button and tabs for anonymous visitors', async () => {
    renderPage()
    await screen.findAllByText('Alpha Bootleg')
    expect(screen.queryByRole('button', { name: 'Upload' })).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Mine' })).not.toBeInTheDocument()
  })

  it('switches to the "Mine" tab for a signed-in user', async () => {
    mockUser = { id: 'owner1', username: 'me' }
    renderPage()
    await screen.findAllByText('Alpha Bootleg')

    fireEvent.click(screen.getByRole('tab', { name: 'Mine' }))

    await waitFor(() => expect(myMashups).toHaveBeenCalled())
    expect(await screen.findByText('My Only Mix')).toBeInTheDocument()
  })

  it('shows an empty state when there are no mashups', async () => {
    listMashups.mockResolvedValue([])
    renderPage()
    expect(await screen.findByText('No mashups found.')).toBeInTheDocument()
  })
})
