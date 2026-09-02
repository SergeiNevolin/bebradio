import { render, screen, act, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import Player from '../components/Player'
import type { Track } from '../types'

const mockTrack: Track = {
  id: 'abc123',
  title: 'Test Song',
  artist: 'Test Artist',
  url: 'https://example.com/audio.mp3',
  thumbnail: 'https://example.com/thumb.jpg',
  duration: 210,
  added_by: 'Alice',
}

const defaultPlayerProps = {
  likes: 0,
  dislikes: 0,
  userVote: 0 as 1 | -1 | 0,
  onVote: vi.fn(),
  onSkipVote: vi.fn(),
  skipVoters: [],
  currentUserId: 'user1',
}

describe('Player', () => {
  it('shows empty message when no track', () => {
    render(<Player track={null} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.getByText('Add a track to start listening together')).toBeInTheDocument()
  })

  it('renders track info', () => {
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.getByText('Test Song')).toBeInTheDocument()
    expect(screen.getByText('Test Artist')).toBeInTheDocument()
    expect(screen.getByText('Added by Alice')).toBeInTheDocument()
  })

  it('renders thumbnail img with correct src', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    const img = container.querySelector('.player-thumb') as HTMLImageElement
    expect(img).toBeInTheDocument()
    expect(img.src).toBe('https://example.com/thumb.jpg')
  })

  it('has audio element', () => {
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(document.querySelector('audio')).toBeInTheDocument()
  })

  it('renders progress bar', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(container.querySelector('.progress-bar')).toBeInTheDocument()
    expect(container.querySelector('.progress-fill')).toBeInTheDocument()
  })

  it('renders volume control', () => {
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.getByTitle('Mute')).toBeInTheDocument()
    expect(document.querySelector('.volume-slider')).toBeInTheDocument()
  })

  it('does not render prev/next buttons', () => {
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.queryByTitle('Previous')).not.toBeInTheDocument()
    expect(screen.queryByTitle('Next')).not.toBeInTheDocument()
  })

  it('does not render controls when no track', () => {
    render(<Player track={null} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.queryByTitle('Mute')).not.toBeInTheDocument()
  })

  it('renders vote buttons when track is present', () => {
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.getByText(/👍/)).toBeInTheDocument()
    expect(screen.getByText(/👎/)).toBeInTheDocument()
    expect(screen.getByText(/Skip/)).toBeInTheDocument()
  })

  it('does not render vote buttons when no track', () => {
    render(<Player track={null} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.queryByText(/👍/)).not.toBeInTheDocument()
  })
})

describe('Player vote buttons', () => {
  it('sends vote=1 when clicking like with no user vote', () => {
    const onVote = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} onVote={onVote} />)
    fireEvent.click(screen.getByText(/👍/))
    expect(onVote).toHaveBeenCalledWith('abc123', 1)
  })

  it('sends vote=0 when clicking like if already liked (userVote=1)', () => {
    const onVote = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} userVote={1} onVote={onVote} />)
    fireEvent.click(screen.getByText(/👍/))
    expect(onVote).toHaveBeenCalledWith('abc123', 0)
  })

  it('sends vote=-1 when clicking dislike with no user vote', () => {
    const onVote = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} onVote={onVote} />)
    fireEvent.click(screen.getByText(/👎/))
    expect(onVote).toHaveBeenCalledWith('abc123', -1)
  })

  it('sends vote=0 when clicking dislike if already disliked (userVote=-1)', () => {
    const onVote = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} userVote={-1} onVote={onVote} />)
    fireEvent.click(screen.getByText(/👎/))
    expect(onVote).toHaveBeenCalledWith('abc123', 0)
  })

  it('sends vote=1 when switching from dislike to like', () => {
    const onVote = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} userVote={-1} onVote={onVote} />)
    fireEvent.click(screen.getByText(/👍/))
    expect(onVote).toHaveBeenCalledWith('abc123', 1)
  })

  it('sends vote=-1 when switching from like to dislike', () => {
    const onVote = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} userVote={1} onVote={onVote} />)
    fireEvent.click(screen.getByText(/👎/))
    expect(onVote).toHaveBeenCalledWith('abc123', -1)
  })

  it('highlights like button when userVote=1', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} userVote={1} />)
    const likeBtn = container.querySelector('.vote-like-group .vote-btn')
    expect(likeBtn).toHaveClass('vote-btn-active')
  })

  it('highlights dislike button when userVote=-1', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} userVote={-1} />)
    const dislikeBtn = container.querySelectorAll('.vote-like-group .vote-btn')[1]
    expect(dislikeBtn).toHaveClass('vote-btn-active-down')
  })

  it('sends vote even when others already voted', () => {
    const onVote = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={5} dislikes={3} userVote={0} onVote={onVote} />)
    fireEvent.click(screen.getByText(/👍/))
    expect(onVote).toHaveBeenCalledWith('abc123', 1)
  })
})

describe('Player vote scale', () => {
  it('renders vote scale bar', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(container.querySelector('.vote-scale')).toBeInTheDocument()
    expect(container.querySelector('.vote-bar')).toBeInTheDocument()
  })

  it('shows empty bar when no votes', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(container.querySelector('.vote-bar-like')).not.toBeInTheDocument()
    expect(container.querySelector('.vote-bar-dislike')).not.toBeInTheDocument()
  })

  it('shows full green bar when all likes', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={10} dislikes={0} />)
    const like = container.querySelector('.vote-bar-like') as HTMLElement
    expect(like.style.width).toBe('100%')
    expect(container.querySelector('.vote-bar-dislike')).not.toBeInTheDocument()
  })

  it('shows full red bar when all dislikes', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={0} dislikes={10} />)
    const dislike = container.querySelector('.vote-bar-dislike') as HTMLElement
    expect(dislike.style.width).toBe('100%')
    expect(container.querySelector('.vote-bar-like')).not.toBeInTheDocument()
  })

  it('shows 75% green and 25% red with 3 likes 1 dislike', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={3} dislikes={1} />)
    const like = container.querySelector('.vote-bar-like') as HTMLElement
    const dislike = container.querySelector('.vote-bar-dislike') as HTMLElement
    expect(like.style.width).toBe('75%')
    expect(dislike.style.width).toBe('25%')
  })

  it('shows 25% green and 75% red with 1 like 3 dislikes', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={1} dislikes={3} />)
    const like = container.querySelector('.vote-bar-like') as HTMLElement
    const dislike = container.querySelector('.vote-bar-dislike') as HTMLElement
    expect(like.style.width).toBe('25%')
    expect(dislike.style.width).toBe('75%')
  })

  it('shows 50/50 with equal votes', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={5} dislikes={5} />)
    const like = container.querySelector('.vote-bar-like') as HTMLElement
    const dislike = container.querySelector('.vote-bar-dislike') as HTMLElement
    expect(like.style.width).toBe('50%')
    expect(dislike.style.width).toBe('50%')
  })

  it('scale is between like and dislike buttons', () => {
    const { container } = render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    const likeGroup = container.querySelector('.vote-like-group')
    const scale = container.querySelector('.vote-scale')
    const skipBtn = container.querySelector('.vote-skip')
    const buttons = container.querySelector('.vote-buttons')
    const children = Array.from(buttons!.children)
    expect(children.indexOf(likeGroup!)).toBeLessThan(children.indexOf(scale!))
    expect(children.indexOf(scale!)).toBeLessThan(children.indexOf(skipBtn!))
  })

  it('resets to empty when track changes and new track has no votes', () => {
    const { container, rerender } = render(
      <Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={5} dislikes={3} />
    )
    expect(container.querySelector('.vote-bar-like')).toBeInTheDocument()

    const newTrack: Track = { ...mockTrack, id: 'new1', title: 'New Song' }
    rerender(
      <Player track={newTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} likes={0} dislikes={0} />
    )
    expect(container.querySelector('.vote-bar-like')).not.toBeInTheDocument()
    expect(container.querySelector('.vote-bar-dislike')).not.toBeInTheDocument()
  })

  it('does not show the tap-to-play prompt when autoplay is allowed', () => {
    render(<Player track={mockTrack} isPlaying={true} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.queryByRole('button', { name: /enable sound/i })).not.toBeInTheDocument()
  })

  it('shows a tap-to-play prompt when the browser blocks autoplay, and clears it on tap', async () => {
    const play = HTMLMediaElement.prototype.play as ReturnType<typeof vi.fn>
    play.mockRejectedValueOnce(new Error('NotAllowedError'))

    render(<Player track={mockTrack} isPlaying={true} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)

    const btn = await screen.findByRole('button', { name: /enable sound/i })
    fireEvent.click(btn)

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /enable sound/i })).not.toBeInTheDocument()
    )
  })

  it('tears down audio when the queue empties (last track skipped)', () => {
    const pauseSpy = vi.spyOn(HTMLMediaElement.prototype, 'pause')
    const { container, rerender } = render(
      <Player track={mockTrack} isPlaying={true} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />
    )
    const audio = container.querySelector('audio') as HTMLAudioElement
    expect(audio.getAttribute('src')).toBe(mockTrack.url)

    pauseSpy.mockClear()
    rerender(
      <Player track={null} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />
    )

    expect(pauseSpy).toHaveBeenCalled()
    expect(audio.getAttribute('src')).toBeNull()
  })

  it('stops audio when unmounted (leaving the room)', () => {
    const pauseSpy = vi.spyOn(HTMLMediaElement.prototype, 'pause')
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load')
    const { container, unmount } = render(
      <Player track={mockTrack} isPlaying={true} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />
    )
    const audio = container.querySelector('audio') as HTMLAudioElement
    expect(audio.getAttribute('src')).toBe(mockTrack.url)

    pauseSpy.mockClear()
    loadSpy.mockClear()
    unmount()

    expect(pauseSpy).toHaveBeenCalled()
    expect(loadSpy).toHaveBeenCalled()
    expect(audio.getAttribute('src')).toBeNull()
  })
})

function countSyncCalls(fn: ReturnType<typeof vi.fn>) {
  return fn.mock.calls.filter((c: any[]) => c[0] === 'sync').length
}

function mockAudioPlaying() {
  const originalCreateElement = document.createElement.bind(document)
  document.createElement = ((tag: string) => {
    const el = originalCreateElement(tag)
    if (tag === 'audio') {
      Object.defineProperty(el, 'paused', { value: false, writable: true, configurable: true })
      Object.defineProperty(el, 'currentTime', { value: 0, writable: true, configurable: true })
      Object.defineProperty(el, 'play', { value: vi.fn().mockResolvedValue(undefined), writable: true, configurable: true })
      Object.defineProperty(el, 'pause', { value: vi.fn(), writable: true, configurable: true })
    }
    return el
  }) as typeof document.createElement
}

describe('Player sync', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockAudioPlaying()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('sends periodic sync when playing', () => {
    const onPlayback = vi.fn()
    render(<Player track={mockTrack} isPlaying={true} position={0} onPlayback={onPlayback} {...defaultPlayerProps} />)

    act(() => {
      vi.advanceTimersByTime(5000)
    })

    expect(countSyncCalls(onPlayback)).toBeGreaterThanOrEqual(1)
  })

  it('does not send sync when paused', () => {
    const onPlayback = vi.fn()
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={onPlayback} {...defaultPlayerProps} />)

    act(() => {
      vi.advanceTimersByTime(5000)
    })

    expect(countSyncCalls(onPlayback)).toBe(0)
  })

  it('sends sync every 5 seconds', () => {
    const onPlayback = vi.fn()
    render(<Player track={mockTrack} isPlaying={true} position={0} onPlayback={onPlayback} {...defaultPlayerProps} />)

    act(() => {
      vi.advanceTimersByTime(15000)
    })

    expect(countSyncCalls(onPlayback)).toBeGreaterThanOrEqual(2)
  })

  it('stops sync when paused', () => {
    const onPlayback = vi.fn()
    const { rerender } = render(
      <Player track={mockTrack} isPlaying={true} position={0} onPlayback={onPlayback} {...defaultPlayerProps} />
    )

    act(() => {
      vi.advanceTimersByTime(5000)
    })

    const countAfterPlay = countSyncCalls(onPlayback)
    expect(countAfterPlay).toBeGreaterThanOrEqual(1)

    rerender(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={onPlayback} {...defaultPlayerProps} />)

    act(() => {
      vi.advanceTimersByTime(10000)
    })

    expect(countSyncCalls(onPlayback)).toBe(countAfterPlay)
  })
})

describe('Player karaoke toggle', () => {
  beforeEach(() => {
    Element.prototype.scrollIntoView = vi.fn()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ available: false, cues: [] }) }),
    )
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('hides the karaoke button when no roomId is given', () => {
    render(<Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} />)
    expect(screen.queryByRole('button', { name: /karaoke/i })).not.toBeInTheDocument()
  })

  it('shows the karaoke button and opens the panel on click', async () => {
    render(
      <Player track={mockTrack} isPlaying={false} position={0} onPlayback={vi.fn()} {...defaultPlayerProps} roomId="ROOM1" />
    )
    const btn = screen.getByRole('button', { name: /karaoke/i })
    expect(btn).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(btn)
    expect(btn).toHaveAttribute('aria-pressed', 'true')
    expect(await screen.findByText(/no lyrics for this track/i)).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/rooms/ROOM1/lyrics')
  })
})
