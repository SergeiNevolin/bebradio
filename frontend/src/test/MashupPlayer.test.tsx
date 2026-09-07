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
    likes: 0,
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

function renderPlayer(over: Partial<MashupPlayerState> = {}, props: Partial<Parameters<typeof MashupPlayer>[0]> = {}) {
  const player = makePlayer(over)
  const handlers = {
    onExpand: vi.fn(),
    onToggleQueue: vi.fn(),
    queueOpen: false,
    ...props,
  }
  render(<MashupPlayer player={player} {...handlers} />)
  return { player, ...handlers }
}

describe('MashupPlayer', () => {
  it('is hidden when nothing is selected', () => {
    const player = makePlayer({ current: null, list: [] })
    const { container } = render(
      <MashupPlayer player={player} onExpand={vi.fn()} queueOpen={false} onToggleQueue={vi.fn()} />,
    )
    expect(container.firstChild).toHaveAttribute('hidden')
  })

  it('shows the current track and transport controls', () => {
    renderPlayer()
    expect(screen.getByText('Night Bootleg')).toBeInTheDocument()
    expect(screen.getByText('DJ Test')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Play' })).toBeInTheDocument()
  })

  it('calls toggle when play/pause is clicked', () => {
    const { player } = renderPlayer()
    fireEvent.click(screen.getByRole('button', { name: 'Play' }))
    expect(player.toggle).toHaveBeenCalled()
  })

  it('calls next/prev and disables prev at the start of the list', () => {
    const { player } = renderPlayer({ index: 0, list: [makeMashup(), makeMashup({ id: 'm2' })] })
    expect(screen.getByRole('button', { name: 'Previous mashup' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Next mashup' }))
    expect(player.next).toHaveBeenCalled()
  })

  it('keeps Next enabled at the end of the list when repeat is "all"', () => {
    renderPlayer({ index: 1, list: [makeMashup(), makeMashup({ id: 'm2' })], repeat: 'all' })
    expect(screen.getByRole('button', { name: 'Next mashup' })).toBeEnabled()
  })

  it('renders an interactive seek slider that can be nudged with the keyboard', () => {
    const { player } = renderPlayer()
    const slider = screen.getByRole('slider', { name: 'Playback position' })
    fireEvent.keyDown(slider, { key: 'ArrowRight' })
    expect(player.seek).toHaveBeenCalledWith(25)
  })

  it('toggles shuffle from the transport', () => {
    const { player } = renderPlayer()
    fireEvent.click(screen.getByRole('button', { name: 'Shuffle' }))
    expect(player.toggleShuffle).toHaveBeenCalled()
  })

  it('calls cycleRepeat and relabels the button by mode', () => {
    const { player } = renderPlayer({ repeat: 'off' })
    fireEvent.click(screen.getByRole('button', { name: 'Repeat off' }))
    expect(player.cycleRepeat).toHaveBeenCalled()
  })

  it('swaps in the repeat-one glyph and label when repeat is "one"', () => {
    renderPlayer({ repeat: 'one' })
    const btn = screen.getByRole('button', { name: 'Repeat one' })
    expect(btn).toHaveAttribute('aria-pressed', 'true')
  })

  it('fires onExpand from the expand button', () => {
    const { onExpand } = renderPlayer()
    fireEvent.click(screen.getByRole('button', { name: 'Expand player' }))
    expect(onExpand).toHaveBeenCalled()
  })

  it('toggles the queue panel and shows the remaining-track count', () => {
    const list = [makeMashup(), makeMashup({ id: 'm2' }), makeMashup({ id: 'm3' }), makeMashup({ id: 'm4' })]
    const { onToggleQueue } = renderPlayer({ index: 1, list }, { queueOpen: true })
    const queueBtn = screen.getByRole('button', { name: 'Queue' })
    expect(queueBtn).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(queueBtn)
    expect(onToggleQueue).toHaveBeenCalled()
    expect(screen.getByText('2')).toBeInTheDocument()
  })
})
