import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { createRef } from 'react'
import NowPlayingModal from '../components/mashup/NowPlayingModal'
import type { MashupPlayer as MashupPlayerState } from '../hooks/useMashupPlayer'
import type { Mashup } from '../types'

function makeMashup(over: Partial<Mashup> = {}): Mashup {
  return {
    id: 'm1',
    owner_id: 'o1',
    owner_name: 'dj',
    title: 'Midnight Bootleg',
    artist: 'DJ Test',
    duration: 200,
    size_bytes: 0,
    status: 'ready',
    has_cover: false,
    plays: 0,
    likes: 12,
    liked: false,
    created_at: '2026-01-01T00:00:00Z',
    stream_url: '/api/mashups/media/abc',
    ...over,
  }
}

function makePlayer(over: Partial<MashupPlayerState> = {}): MashupPlayerState {
  const current = makeMashup()
  return {
    audioRef: createRef<HTMLAudioElement>(),
    list: [current],
    setList: vi.fn(),
    index: 0,
    current,
    isPlaying: true,
    position: 40,
    duration: 200,
    buffered: 0,
    shuffle: false,
    repeat: 'off',
    volume: 0.7,
    muted: false,
    setVolume: vi.fn(),
    toggleMute: vi.fn(),
    toggleShuffle: vi.fn(),
    cycleRepeat: vi.fn(),
    play: vi.fn(),
    playAt: vi.fn(),
    toggle: vi.fn(),
    next: vi.fn(),
    prev: vi.fn(),
    seek: vi.fn(),
    ...over,
  } as MashupPlayerState
}

function renderModal(over: Partial<MashupPlayerState> = {}, queue: Mashup[] = []) {
  const player = makePlayer(over)
  const onClose = vi.fn()
  render(
    <NowPlayingModal
      player={player}
      queue={queue}
      onToggleLike={vi.fn()}
      onOpenProfile={vi.fn()}
      onClose={onClose}
    />,
  )
  return { player, onClose }
}

describe('NowPlayingModal', () => {
  it('shows the current track', () => {
    renderModal()
    expect(screen.getByRole('dialog', { name: 'Now playing' })).toBeInTheDocument()
    expect(screen.getByText('Midnight Bootleg')).toBeInTheDocument()
  })

  it('closes on the collapse chevron', () => {
    const { onClose } = renderModal()
    fireEvent.click(screen.getByRole('button', { name: 'Collapse player' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('closes on Escape', () => {
    const { onClose } = renderModal()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('plays a track picked from the queue', () => {
    const q = makeMashup({ id: 'm2', title: 'Second Bootleg' })
    const { player } = renderModal({}, [q])
    fireEvent.click(screen.getByRole('button', { name: /Second Bootleg/ }))
    expect(player.play).toHaveBeenCalledWith(q)
  })

  it('drives shuffle and repeat through the hook', () => {
    const { player } = renderModal()
    fireEvent.click(screen.getByRole('button', { name: 'Shuffle' }))
    fireEvent.click(screen.getByRole('button', { name: 'Repeat off' }))
    expect(player.toggleShuffle).toHaveBeenCalled()
    expect(player.cycleRepeat).toHaveBeenCalled()
  })
})
