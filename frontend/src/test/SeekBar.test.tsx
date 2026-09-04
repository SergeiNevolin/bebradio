import { render, screen } from '@testing-library/react'
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
    const fill = container.querySelector('.seek-fill') as HTMLElement
    expect(fill.style.width).toBe('25%')
  })

  it('exposes an accessible meter with value range', () => {
    render(<SeekBar position={30} duration={120} />)
    const meter = screen.getByRole('meter', { name: 'Playback position' })
    expect(meter).toHaveAttribute('aria-valuemax', '120')
    expect(meter).toHaveAttribute('aria-valuenow', '30')
  })
})
