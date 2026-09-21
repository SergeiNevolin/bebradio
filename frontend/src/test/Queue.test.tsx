import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import Queue from '../components/Queue'
import type { Track } from '../types'

const base = {
  source: 'youtube' as const,
  thumbnail: '',
  url: '',
  duration: 0,
  added_by: '',
  owner_id: '',
  size_bytes: 0,
  status: 'ready' as const,
  has_cover: false,
  plays: 0,
  likes: 0,
  created_at: '',
}
const tracks: Track[] = [
  { id: '1', title: 'Song A', artist: 'Artist A', ...base },
  { id: '2', title: 'Song B', artist: 'Artist B', ...base },
  { id: '3', title: 'Song C', artist: 'Artist C', ...base },
]

describe('Queue', () => {
  it('shows empty message when no tracks', () => {
    render(<Queue queue={[]} currentIndex={0} />)
    expect(screen.getByText('No tracks yet. Add something above.')).toBeInTheDocument()
  })

  it('renders track list', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('Song A')).toBeInTheDocument()
    expect(screen.getByText('Song B')).toBeInTheDocument()
    expect(screen.getByText('Song C')).toBeInTheDocument()
  })

  it('shows queue count', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('Queue (3)')).toBeInTheDocument()
  })

  it('highlights current track', () => {
    const { container } = render(<Queue queue={tracks} currentIndex={1} />)
    const items = container.querySelectorAll('.queueItem')
    expect(items[1]).toHaveClass('queueItemActive')
    expect(items[0]).not.toHaveClass('queueItemActive')
  })

  it('renders track numbers', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('renders artist names', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('Artist A')).toBeInTheDocument()
    expect(screen.getByText('Artist B')).toBeInTheDocument()
  })

  it('shows a radio search message instead of the empty hint while searching', () => {
    render(<Queue queue={[]} currentIndex={0} searching />)
    expect(screen.getByText(/radio is finding tracks/i)).toBeInTheDocument()
    expect(screen.queryByText('No tracks yet. Add something above.')).not.toBeInTheDocument()
  })

  it('shows a footer search hint below a non-empty queue while searching', () => {
    render(<Queue queue={tracks} currentIndex={0} searching />)
    expect(screen.getByText(/radio is finding more tracks/i)).toBeInTheDocument()
  })

  it('shows no search hint when not searching', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.queryByText(/radio is finding/i)).not.toBeInTheDocument()
  })

  it('shows a badge per track source', () => {
    const mixed: Track[] = [
      { ...tracks[0], source: 'youtube' },
      { ...tracks[1], source: 'upload' },
    ]
    render(<Queue queue={mixed} currentIndex={0} />)
    expect(screen.getByText('YouTube')).toBeInTheDocument()
    expect(screen.getByText('Mashup')).toBeInTheDocument()
  })

  it('falls back to the raw source string for future providers', () => {
    render(<Queue queue={[{ ...tracks[0], source: 'spotify' }]} currentIndex={0} />)
    expect(screen.getByText('spotify')).toBeInTheDocument()
  })
})
