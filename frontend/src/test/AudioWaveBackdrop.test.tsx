import { render } from '@testing-library/react'
import { describe, it, expect, vi, afterEach } from 'vitest'
import AudioWaveBackdrop from '../components/AudioWaveBackdrop'

function stubCanvas2d() {
  const ctx = {
    setTransform: vi.fn(),
    clearRect: vi.fn(),
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    closePath: vi.fn(),
    fill: vi.fn(),
    createLinearGradient: vi.fn(() => ({ addColorStop: vi.fn() })),
    fillStyle: '',
    globalCompositeOperation: 'source-over',
  }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    ctx as unknown as CanvasRenderingContext2D,
  )
  return ctx
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AudioWaveBackdrop', () => {
  it('renders nothing until a track is active', () => {
    const { container } = render(<AudioWaveBackdrop active={false} isPlaying={false} seed="" />)
    expect(container.querySelector('.wave-backdrop')).not.toBeInTheDocument()
  })

  it('renders a non-interactive canvas backdrop when active', () => {
    stubCanvas2d()
    const { container } = render(<AudioWaveBackdrop active isPlaying seed="track-1" />)
    const el = container.querySelector('.wave-backdrop') as HTMLElement
    expect(el).toBeInTheDocument()
    expect(el).toHaveAttribute('aria-hidden', 'true')
    expect(el.querySelector('canvas.wave-canvas')).toBeInTheDocument()
  })

  it('starts an animation frame loop while active', () => {
    stubCanvas2d()
    const raf = vi.spyOn(window, 'requestAnimationFrame')
    render(<AudioWaveBackdrop active isPlaying seed="s" />)
    expect(raf).toHaveBeenCalled()
  })

  it('cancels the animation frame loop on unmount', () => {
    stubCanvas2d()
    const cancel = vi.spyOn(window, 'cancelAnimationFrame')
    const { unmount } = render(<AudioWaveBackdrop active isPlaying seed="s" />)
    unmount()
    expect(cancel).toHaveBeenCalled()
  })

  it('does not throw when the canvas 2d context is unavailable', () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
    expect(() => render(<AudioWaveBackdrop active isPlaying seed="s" />)).not.toThrow()
  })
})
