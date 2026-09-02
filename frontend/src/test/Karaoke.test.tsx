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
    mockLyrics({ available: true, cues })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(screen.getByText(/loading lyrics/i)).toBeInTheDocument()
  })

  it('renders every lyric line once loaded', async () => {
    mockLyrics({ available: true, auto: false, cues })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText('first line')).toBeInTheDocument()
    expect(screen.getByText('second line')).toBeInTheDocument()
    expect(screen.getByText('third line')).toBeInTheDocument()
  })

  it('marks the line matching the current playback time as active', async () => {
    mockLyrics({ available: true, cues })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={3.5} />)
    const active = await screen.findByText('second line')
    expect(active).toHaveClass('is-active')
    expect(screen.getByText('first line')).toHaveClass('is-past')
    expect(screen.getByText('third line')).not.toHaveClass('is-active')
  })

  it('fetches the room lyrics endpoint', async () => {
    mockLyrics({ available: true, cues })
    render(<Karaoke roomId="ROOM9" trackId="t1" currentTime={0} />)
    await screen.findByText('first line')
    expect(fetch).toHaveBeenCalledWith('/api/rooms/ROOM9/lyrics')
  })

  it('shows an empty state when the track has no lyrics', async () => {
    mockLyrics({ available: false, cues: [] })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText(/no lyrics for this track/i)).toBeInTheDocument()
  })

  it('shows an error state when the request fails', async () => {
    mockLyrics({}, false)
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText(/couldn.t load lyrics/i)).toBeInTheDocument()
  })

  it('notes when captions are auto-generated', async () => {
    mockLyrics({ available: true, auto: true, cues })
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    expect(await screen.findByText(/auto-generated captions/i)).toBeInTheDocument()
  })

  it('seeks to a line timestamp when it is clicked', async () => {
    mockLyrics({ available: true, cues })
    const onSeek = vi.fn()
    render(<Karaoke roomId="R1" trackId="t1" currentTime={0} onSeek={onSeek} />)
    const line = await screen.findByText('third line')
    line.click()
    expect(onSeek).toHaveBeenCalledWith(5)
  })

  it('re-fetches when the track changes', async () => {
    mockLyrics({ available: true, cues })
    const { rerender } = render(<Karaoke roomId="R1" trackId="t1" currentTime={0} />)
    await screen.findByText('first line')
    expect(fetch).toHaveBeenCalledTimes(1)
    rerender(<Karaoke roomId="R1" trackId="t2" currentTime={0} />)
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2))
  })
})
