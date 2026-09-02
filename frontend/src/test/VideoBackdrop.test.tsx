import { render, act, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import VideoBackdrop from '../components/VideoBackdrop'

const player = {
  loadVideoById: vi.fn(),
  playVideo: vi.fn(),
  pauseVideo: vi.fn(),
  mute: vi.fn(),
  seekTo: vi.fn(),
  getCurrentTime: vi.fn(() => 0),
  destroy: vi.fn(),
}

const PlayerCtor = vi.fn().mockImplementation((_el: unknown, opts: any) => {
  opts.events?.onReady?.({ target: player })
  return player
})

beforeEach(() => {
  vi.clearAllMocks()
  // Pretend the IFrame API is already loaded so loadYouTubeApi resolves at once.
  ;(window as unknown as { YT: unknown }).YT = { Player: PlayerCtor }
})

describe('VideoBackdrop', () => {
  it('renders nothing without a video id', () => {
    const { container } = render(<VideoBackdrop videoId="" isPlaying position={0} />)
    expect(container.querySelector('.video-backdrop')).not.toBeInTheDocument()
  })

  it('renders a non-interactive backdrop container for a video id', () => {
    const { container } = render(
      <VideoBackdrop videoId="dQw4w9WgXcQ" isPlaying position={0} />,
    )
    const el = container.querySelector('.video-backdrop') as HTMLElement
    expect(el).toBeInTheDocument()
    expect(el).toHaveAttribute('aria-hidden', 'true')
    expect(el.querySelector('.video-backdrop-frame')).toBeInTheDocument()
  })

  it('creates a muted player for the given video and follows play state', async () => {
    render(<VideoBackdrop videoId="dQw4w9WgXcQ" isPlaying position={0} />)
    await waitFor(() => expect(PlayerCtor).toHaveBeenCalled())
    const opts = PlayerCtor.mock.calls[0][1] as any
    expect(opts.videoId).toBe('dQw4w9WgXcQ')
    expect(player.mute).toHaveBeenCalled()
    expect(player.playVideo).toHaveBeenCalled()
  })

  it('pauses the clip when the room is paused', async () => {
    render(<VideoBackdrop videoId="dQw4w9WgXcQ" isPlaying={false} position={0} />)
    await waitFor(() => expect(player.pauseVideo).toHaveBeenCalled())
    expect(player.playVideo).not.toHaveBeenCalled()
  })

  it('swaps the clip when the track changes', async () => {
    const { rerender } = render(
      <VideoBackdrop videoId="dQw4w9WgXcQ" isPlaying position={0} />,
    )
    await waitFor(() => expect(PlayerCtor).toHaveBeenCalled())
    rerender(<VideoBackdrop videoId="abcdefghijk" isPlaying position={0} />)
    await waitFor(() => expect(player.loadVideoById).toHaveBeenCalledWith('abcdefghijk'))
  })

  it('destroys the player on unmount', async () => {
    const { unmount } = render(
      <VideoBackdrop videoId="dQw4w9WgXcQ" isPlaying position={0} />,
    )
    await waitFor(() => expect(PlayerCtor).toHaveBeenCalled())
    unmount()
    expect(player.destroy).toHaveBeenCalled()
  })

  it('hides itself when the video cannot be embedded', async () => {
    let optsRef: any
    PlayerCtor.mockImplementationOnce((_el: unknown, opts: any) => {
      optsRef = opts
      return player
    })
    const { container } = render(
      <VideoBackdrop videoId="dQw4w9WgXcQ" isPlaying position={0} />,
    )
    await waitFor(() => expect(optsRef).toBeTruthy())
    act(() => optsRef.events.onError())
    await waitFor(() =>
      expect(container.querySelector('.video-backdrop')).not.toBeInTheDocument(),
    )
  })
})
