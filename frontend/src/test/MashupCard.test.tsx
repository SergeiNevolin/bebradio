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
  canDelete: false,
  canManageCover: false,
  onPlay: vi.fn(),
  onDelete: vi.fn(),
  onToggleLike: vi.fn(),
  onChangeCover: vi.fn(),
}

describe('MashupCard', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the owner and the like count', () => {
    render(<MashupCard mashup={mashup()} {...noop} />)
    expect(screen.getByText('by dj')).toBeInTheDocument()
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

  it('offers a cover control only to the owner and passes the chosen file up', () => {
    const onChangeCover = vi.fn()
    const { rerender } = render(<MashupCard mashup={mashup()} {...noop} onChangeCover={onChangeCover} />)
    expect(screen.queryByRole('button', { name: 'Change cover for Alpha Bootleg' })).not.toBeInTheDocument()

    rerender(<MashupCard mashup={mashup()} {...noop} canManageCover onChangeCover={onChangeCover} />)
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File(['img'], 'cover.png', { type: 'image/png' })
    fireEvent.change(input, { target: { files: [file] } })
    expect(onChangeCover).toHaveBeenCalledWith(file)
  })
})
