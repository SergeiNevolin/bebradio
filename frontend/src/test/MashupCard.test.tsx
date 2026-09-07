import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import MashupCard from '../components/mashup/MashupCard'
import type { Mashup } from '../types'

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
    likes: 2,
    liked: false,
    created_at: '2026-01-01T00:00:00Z',
    stream_url: '/api/mashups/media/aaa',
    ...over,
  }
}

const noop = {
  active: false,
  isPlaying: false,
  canEdit: false,
  onPlay: vi.fn(),
  onToggleLike: vi.fn(),
  onEdit: vi.fn(),
}

describe('MashupCard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the owner and the like count', () => {
    render(<MashupCard mashup={mashup()} {...noop} />)
    expect(screen.getByText(/by dj/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Like Alpha Bootleg' })).toHaveTextContent('2')
  })

  it('reflects the liked state and fires onToggleLike', () => {
    const onToggleLike = vi.fn()
    render(<MashupCard mashup={mashup({ liked: true, likes: 3 })} {...noop} onToggleLike={onToggleLike} />)
    const btn = screen.getByRole('button', { name: 'Unlike Alpha Bootleg' })
    expect(btn).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(btn)
    expect(onToggleLike).toHaveBeenCalled()
  })

  it('plays from the artwork and from a click anywhere on the card', () => {
    const onPlay = vi.fn()
    render(<MashupCard mashup={mashup()} {...noop} onPlay={onPlay} />)
    fireEvent.click(screen.getByRole('button', { name: 'Play Alpha Bootleg' }))
    expect(onPlay).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByText('Alpha Bootleg'))
    expect(onPlay).toHaveBeenCalledTimes(2)
  })

  it('shows a pause affordance on the active, playing card', () => {
    render(<MashupCard mashup={mashup()} {...noop} active isPlaying />)
    expect(screen.getByRole('button', { name: 'Pause Alpha Bootleg' })).toBeInTheDocument()
  })

  it('offers an Edit button only to the owner and passes the intent up', () => {
    const onEdit = vi.fn()
    const { rerender } = render(<MashupCard mashup={mashup()} {...noop} onEdit={onEdit} />)
    expect(screen.queryByRole('button', { name: 'Edit Alpha Bootleg' })).not.toBeInTheDocument()

    rerender(<MashupCard mashup={mashup()} {...noop} canEdit onEdit={onEdit} />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit Alpha Bootleg' }))
    expect(onEdit).toHaveBeenCalled()
  })
})
