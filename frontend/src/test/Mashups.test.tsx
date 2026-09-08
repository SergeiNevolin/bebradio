import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Mashups from '../pages/Mashups'
import type { Mashup } from '../types'

const {
  listMashups,
  myMashups,
  likedMashups,
  getMashup,
  deleteMashup,
  likeMashup,
  unlikeMashup,
  uploadMashupCover,
} = vi.hoisted(() => ({
  listMashups: vi.fn(),
  myMashups: vi.fn(),
  likedMashups: vi.fn(),
  getMashup: vi.fn(),
  deleteMashup: vi.fn(),
  likeMashup: vi.fn(),
  unlikeMashup: vi.fn(),
  uploadMashupCover: vi.fn(),
}))
vi.mock('../lib/api', () => ({
  api: { listMashups, myMashups, likedMashups, getMashup, deleteMashup, likeMashup, unlikeMashup, uploadMashupCover },
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
    owner_name: 'dj',
    title: 'Alpha Bootleg',
    artist: 'DJ A',
    duration: 180,
    size_bytes: 0,
    status: 'ready',
    has_cover: false,
    plays: 0,
    likes: 3,
    liked: false,
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
    listMashups.mockImplementation((_q: string, sort: 'recent' | 'top') =>
      Promise.resolve(
        sort === 'top'
          ? [mashup({ id: 't1', title: 'Top Bootleg', likes: 9 })]
          : [mashup(), mashup({ id: 'm2', title: 'Beta Bootleg' })],
      ),
    )
    myMashups.mockResolvedValue([mashup({ id: 'm9', title: 'My Only Mix', owner_id: 'owner1' })])
    likedMashups.mockResolvedValue([mashup({ id: 'l1', title: 'Liked Bootleg', liked: true, likes: 5 })])
    likeMashup.mockResolvedValue({ likes: 4, liked: true })
    unlikeMashup.mockResolvedValue({ likes: 3, liked: false })
  })

  it('renders the three browse sections', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: 'Загружайте и слушайте мешапы' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Latest' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Top by likes' })).toBeInTheDocument()
    expect(await screen.findAllByText('Top Bootleg')).not.toHaveLength(0)
    // recent + top requested with their sorts
    expect(listMashups).toHaveBeenCalledWith('', 'recent', 12)
    expect(listMashups).toHaveBeenCalledWith('', 'top', 24, 0)
  })

  it('adds the Liked and My mashups sections for a signed-in user', async () => {
    mockUser = { id: 'owner1', username: 'me' }
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Liked' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'My mashups' })).toBeInTheDocument()
    // Titles now show in both the centre shelves and the left library rail.
    expect((await screen.findAllByText('Liked Bootleg')).length).toBeGreaterThan(0)
    expect((await screen.findAllByText('My Only Mix')).length).toBeGreaterThan(0)
  })

  it('hides the upload button and personal sections for anonymous visitors', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Latest' })
    expect(screen.queryByRole('button', { name: 'Upload' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Liked' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'My mashups' })).not.toBeInTheDocument()
  })

  it('collapses to a search result grid while a query is active', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Latest' })

    fireEvent.change(screen.getByLabelText('Search mashups'), { target: { value: 'beta' } })

    await waitFor(() => expect(listMashups).toHaveBeenCalledWith('beta', 'recent', 50))
    expect(await screen.findByRole('heading', { name: 'Search results' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Latest' })).not.toBeInTheDocument()
  })

  it('toggles a like optimistically and settles on the server count', async () => {
    mockUser = { id: 'someone', username: 'me' }
    renderPage()
    const top = (await screen.findByRole('heading', { name: 'Top by likes' })).parentElement as HTMLElement
    const likeBtn = within(top).getByRole('button', { name: 'Like Top Bootleg' })

    fireEvent.click(likeBtn)

    await waitFor(() => expect(likeMashup).toHaveBeenCalledWith('t1'))
    await waitFor(() =>
      expect(within(top).getByRole('button', { name: 'Unlike Top Bootleg' })).toBeInTheDocument(),
    )
  })

  it('asks anonymous visitors to sign in before liking', async () => {
    renderPage()
    const top = (await screen.findByRole('heading', { name: 'Top by likes' })).parentElement as HTMLElement
    fireEvent.click(within(top).getByRole('button', { name: 'Like Top Bootleg' }))
    expect(showToast).toHaveBeenCalledWith('Sign in to like mashups', 'error')
    expect(likeMashup).not.toHaveBeenCalled()
  })

  it('shows the reworked player transport once a track is playing', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Latest' })
    fireEvent.click(screen.getAllByRole('button', { name: 'Play Alpha Bootleg' })[0])

    expect(await screen.findByRole('button', { name: 'Expand player' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Shuffle' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Repeat off' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Queue' })).toBeInTheDocument()
  })

  it('opens the expanded Now Playing overlay from the expand button', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Latest' })
    fireEvent.click(screen.getAllByRole('button', { name: 'Play Alpha Bootleg' })[0])

    fireEvent.click(await screen.findByRole('button', { name: 'Expand player' }))
    expect(screen.getByRole('dialog', { name: 'Now playing' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Collapse player' }))
    expect(screen.queryByRole('dialog', { name: 'Now playing' })).not.toBeInTheDocument()
  })

  it('hides and restores the Now Playing side panel from the queue button', async () => {
    const { container } = renderPage()
    await screen.findByRole('heading', { name: 'Latest' })
    fireEvent.click(screen.getAllByRole('button', { name: 'Play Alpha Bootleg' })[0])

    const panel = () => container.querySelectorAll('aside[aria-label="Now playing"]')
    expect(panel()).toHaveLength(1)

    fireEvent.click(await screen.findByRole('button', { name: 'Queue' }))
    expect(panel()).toHaveLength(0)

    fireEvent.click(screen.getByRole('button', { name: 'Queue' }))
    expect(panel()).toHaveLength(1)
  })
})
