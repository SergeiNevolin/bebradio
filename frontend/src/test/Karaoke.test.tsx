import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import Karaoke from '../components/Karaoke'

const cues = [
  { start: 1, dur: 2, text: 'first line' },
  { start: 3, dur: 2, text: 'second line' },
  { start: 5, dur: 2, text: 'third line' },
]

function mockLyrics(body: unknown, ok = true) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({ ok, json: () => Promise.resolve(body) }),
  )
}

describe('Karaoke', () => {
  beforeEach(() => {
    Element.prototype.scrollIntoView = vi.fn()
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows a loading state before the request resolves', () => {
    mockLyrics({ cues, auto: false })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(screen.getByText(/loading lyrics/i)).toBeInTheDocument()
  })

  it('renders every lyric line once loaded', async () => {
    mockLyrics({ cues, auto: false })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText('first line')).toBeInTheDocument()
    expect(screen.getByText('second line')).toBeInTheDocument()
    expect(screen.getByText('third line')).toBeInTheDocument()
  })

  it('marks the line matching the current playback time as active', async () => {
    mockLyrics({ cues, auto: false })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={3.5} />)
    const active = await screen.findByText('second line')
    expect(active).toHaveClass('karaokeLineActive')
    expect(screen.getByText('first line')).toHaveClass('karaokeLinePast')
    expect(screen.getByText('third line')).not.toHaveClass('karaokeLineActive')
  })

  it('fetches the room lyrics endpoint', async () => {
    mockLyrics({ cues, auto: false })
    render(<Karaoke roomId="ROOM9" trackId="t1" currentTime={0} />)
    await screen.findByText('first line')
    expect(fetch).toHaveBeenCalledWith('/api/rooms/ROOM9/lyrics', {
      headers: { 'Content-Type': 'application/json' },
    })
  })

  it('shows an empty state when the track has no lyrics', async () => {
    mockLyrics({ cues: [], available: false, auto: false })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText(/no lyrics for this track/i)).toBeInTheDocument()
  })

  it('shows an error state when the request fails', async () => {
    mockLyrics({}, false)
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText(/couldn.t load lyrics/i)).toBeInTheDocument()
  })

  it('renders lines with no auto-generated note when not flagged', async () => {
    mockLyrics({ cues, auto: false })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText('first line')).toBeInTheDocument()
    expect(screen.queryByText(/auto-generated captions/i)).not.toBeInTheDocument()
  })

  it('re-fetches when the track changes', async () => {
    mockLyrics({ cues, auto: false })
    const { rerender } = render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    await screen.findByText('first line')
    expect(fetch).toHaveBeenCalledTimes(1)
    rerender(<Karaoke roomId="R1" trackId="t2" currentTime={0} />)
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2))
  })
})
