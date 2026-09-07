import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import SeekBar from '../components/player/SeekBar'

describe('SeekBar', () => {
  it('renders current and total time labels', () => {
    render(<SeekBar position={42} duration={195} />)
    expect(screen.getByText('0:42')).toBeInTheDocument()
    expect(screen.getByText('3:15')).toBeInTheDocument()
  })

  it('fills proportionally to the position', () => {
    const { container } = render(<SeekBar position={50} duration={200} />)
    const fill = container.querySelector('.seekFill') as HTMLElement
    expect(fill.style.width).toBe('25%')
  })

  it('exposes an accessible meter with value range', () => {
    render(<SeekBar position={30} duration={120} />)
    const meter = screen.getByRole('meter', { name: 'Playback position' })
    expect(meter).toHaveAttribute('aria-valuemax', '120')
    expect(meter).toHaveAttribute('aria-valuenow', '30')
  })

  it('is a slider once onSeek is provided', () => {
    render(<SeekBar position={0} duration={120} onSeek={vi.fn()} />)
    expect(screen.getByRole('slider', { name: 'Playback position' })).toBeInTheDocument()
  })

  it('draws the buffered portion when given', () => {
    const { container } = render(<SeekBar position={0} duration={120} buffered={30} />)
    const buf = container.querySelector('.seekBuffer') as HTMLElement
    expect(buf).not.toBeNull()
    expect(buf.style.width).toBe('25%')
  })

  it('omits the buffer bar without a buffered prop', () => {
    const { container } = render(<SeekBar position={0} duration={120} />)
    expect(container.querySelector('.seekBuffer')).toBeNull()
  })

  it('shows a time tooltip on hover and hides it on leave (interactive only)', () => {
    const { container } = render(<SeekBar position={0} duration={120} onSeek={vi.fn()} />)
    const track = screen.getByRole('slider', { name: 'Playback position' })
    track.getBoundingClientRect = () =>
      ({ left: 0, width: 200, top: 0, right: 200, bottom: 16, height: 16, x: 0, y: 0 }) as DOMRect

    fireEvent.pointerMove(track, { clientX: 100 })
    expect(container.querySelector('.seekTooltip')?.textContent).toBe('1:00')

    fireEvent.pointerLeave(track)
    expect(container.querySelector('.seekTooltip')).toBeNull()
  })

  it('does not show a tooltip in meter mode', () => {
    const { container } = render(<SeekBar position={0} duration={120} />)
    const track = screen.getByRole('meter', { name: 'Playback position' })
    fireEvent.pointerMove(track, { clientX: 100 })
    expect(container.querySelector('.seekTooltip')).toBeNull()
  })
})
