import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { createRef } from 'react'
import MashupPlayer from '../components/mashup/MashupPlayer'
import type { MashupPlayer as MashupPlayerState } from '../hooks/useMashupPlayer'
import type { Mashup } from '../types'

function makeMashup(over: Partial<Mashup> = {}): Mashup {
  return {
    id: 'm1',
    owner_id: 'o1',
    title: 'Night Bootleg',
    artist: 'DJ Test',
    duration: 200,
    size_bytes: 0,
    status: 'ready',
    has_cover: false,
    plays: 0,
    created_at: '2026-01-01T00:00:00Z',
    stream_url: '/api/mashups/media/abc',
    ...over,
  }
}

function makePlayer(over: Partial<MashupPlayerState> = {}): MashupPlayerState {
  return {
    audioRef: createRef<HTMLAudioElement>(),
    list: [makeMashup()],
    setList: vi.fn(),
    index: 0,
    current: makeMashup(),
    isPlaying: false,
    position: 20,
    duration: 200,
    volume: 0.7,
    muted: false,
    setVolume: vi.fn(),
    toggleMute: vi.fn(),
    play: vi.fn(),
    playAt: vi.fn(),
    toggle: vi.fn(),
    next: vi.fn(),
    prev: vi.fn(),
    seek: vi.fn(),
    ...over,
  } as MashupPlayerState
}

describe('MashupPlayer', () => {
  it('is hidden when nothing is selected', () => {
    const player = makePlayer({ current: null, list: [] })
    const { container } = render(<MashupPlayer player={player} />)
    expect(container.firstChild).toHaveAttribute('hidden')
  })

  it('shows the current track and transport controls', () => {
    render(<MashupPlayer player={makePlayer()} />)
    expect(screen.getByText('Night Bootleg')).toBeInTheDocument()
    expect(screen.getByText('DJ Test')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Play' })).toBeInTheDocument()
  })

  it('calls toggle when play/pause is clicked', () => {
    const player = makePlayer()
    render(<MashupPlayer player={player} />)
    fireEvent.click(screen.getByRole('button', { name: 'Play' }))
    expect(player.toggle).toHaveBeenCalled()
  })

  it('calls next/prev and disables prev at the start of the list', () => {
    const player = makePlayer({ index: 0, list: [makeMashup(), makeMashup({ id: 'm2' })] })
    render(<MashupPlayer player={player} />)
    expect(screen.getByRole('button', { name: 'Previous mashup' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Next mashup' }))
    expect(player.next).toHaveBeenCalled()
  })

  it('renders an interactive seek slider that can be nudged with the keyboard', () => {
    const player = makePlayer()
    render(<MashupPlayer player={player} />)
    const slider = screen.getByRole('slider', { name: 'Playback position' })
    fireEvent.keyDown(slider, { key: 'ArrowRight' })
    expect(player.seek).toHaveBeenCalledWith(25)
  })
})
