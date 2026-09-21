import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
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

  it('falls back to placeholder art and Unknown artist', () => {
    const bare: Track = {
      id: '9', title: 'No Cover', artist: '', source: 'upload',
      thumbnail: '', url: '', duration: 0, added_by: '', owner_id: '',
      size_bytes: 0, status: 'ready', has_cover: false, plays: 0, likes: 0,
      created_at: '',
    }
    const { container } = render(<Queue queue={[bare]} currentIndex={0} />)
    expect(container.querySelector('img')).not.toBeInTheDocument()
    expect(screen.getByText('N')).toBeInTheDocument()
    expect(screen.getByText('Unknown artist')).toBeInTheDocument()
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

  it('hides the import button without canImport', () => {
    render(<Queue queue={tracks} currentIndex={0} onImport={vi.fn()} />)
    expect(screen.queryByRole('button', { name: 'Save to bebradio' })).not.toBeInTheDocument()
  })

  it('hides the import button on library tracks', () => {
    const lib: Track = { ...tracks[0], source: 'upload' }
    render(<Queue queue={[lib]} currentIndex={0} canImport onImport={vi.fn()} />)
    expect(screen.queryByRole('button', { name: 'Save to bebradio' })).not.toBeInTheDocument()
  })

  it('offers import on YouTube tracks for admins and calls back with the id', async () => {
    const onImport = vi.fn().mockResolvedValue(undefined)
    render(<Queue queue={tracks} currentIndex={0} canImport onImport={onImport} />)
    const buttons = screen.getAllByRole('button', { name: 'Save to bebradio' })
    expect(buttons).toHaveLength(3)
    fireEvent.click(buttons[0])
    await waitFor(() => {
      expect(onImport).toHaveBeenCalledWith('1')
    })
  })

  it('disables import buttons while an import is in flight', async () => {
    let resolveImport: () => void = () => {}
    const onImport = vi.fn().mockImplementation(() => new Promise<void>((r) => { resolveImport = r }))
    render(<Queue queue={tracks} currentIndex={0} canImport onImport={onImport} />)
    fireEvent.click(screen.getAllByRole('button', { name: /Save to bebradio|Saving/ })[0])
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled()
    })
    resolveImport()
    await waitFor(() => {
      const buttons = screen.getAllByRole('button', { name: 'Save to bebradio' })
      expect(buttons).toHaveLength(3)
      for (const b of buttons) expect(b).not.toBeDisabled()
    })
  })
})
