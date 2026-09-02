import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import VolumeControl from '../components/player/VolumeControl'

const noop = {
  onVolume: vi.fn(),
  onToggleMute: vi.fn(),
}

describe('VolumeControl', () => {
  it('always renders the slider (no hover needed)', () => {
    render(<VolumeControl volume={0.7} muted={false} {...noop} />)
    expect(screen.getByRole('slider', { name: 'Volume' })).toBeInTheDocument()
  })

  it('reflects the current volume on the slider', () => {
    render(<VolumeControl volume={0.4} muted={false} {...noop} />)
    expect(screen.getByRole('slider', { name: 'Volume' })).toHaveValue('0.4')
  })

  it('shows the slider at zero while muted', () => {
    render(<VolumeControl volume={0.8} muted {...noop} />)
    expect(screen.getByRole('slider', { name: 'Volume' })).toHaveValue('0')
  })

  it('calls onVolume with the new level on change', () => {
    const onVolume = vi.fn()
    render(<VolumeControl volume={0.5} muted={false} onVolume={onVolume} onToggleMute={vi.fn()} />)
    fireEvent.change(screen.getByRole('slider', { name: 'Volume' }), { target: { value: '0.25' } })
    expect(onVolume).toHaveBeenCalledWith(0.25)
  })

  it('toggles mute from the icon button', () => {
    const onToggleMute = vi.fn()
    render(<VolumeControl volume={0.5} muted={false} onVolume={vi.fn()} onToggleMute={onToggleMute} />)
    fireEvent.click(screen.getByRole('button', { name: 'Mute' }))
    expect(onToggleMute).toHaveBeenCalled()
  })

  it('labels the button Unmute when silent', () => {
    render(<VolumeControl volume={0} muted={false} {...noop} />)
    expect(screen.getByRole('button', { name: 'Unmute' })).toBeInTheDocument()
  })
})
