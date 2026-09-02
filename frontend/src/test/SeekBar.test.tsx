import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import SeekBar from '../components/player/SeekBar'

describe('SeekBar', () => {
  it('renders current and total time labels', () => {
    render(<SeekBar position={42} duration={195} onSeek={vi.fn()} />)
    expect(screen.getByText('0:42')).toBeInTheDocument()
    expect(screen.getByText('3:15')).toBeInTheDocument()
  })

  it('fills proportionally to the position', () => {
    const { container } = render(<SeekBar position={50} duration={200} onSeek={vi.fn()} />)
    const fill = container.querySelector('.seek-fill') as HTMLElement
    expect(fill.style.width).toBe('25%')
  })

  it('exposes an accessible slider with value range', () => {
    render(<SeekBar position={30} duration={120} onSeek={vi.fn()} />)
    const slider = screen.getByRole('slider', { name: 'Seek' })
    expect(slider).toHaveAttribute('aria-valuemax', '120')
    expect(slider).toHaveAttribute('aria-valuenow', '30')
  })

  it('seeks forward and back with arrow keys', () => {
    const onSeek = vi.fn()
    render(<SeekBar position={30} duration={120} onSeek={onSeek} />)
    const slider = screen.getByRole('slider', { name: 'Seek' })
    fireEvent.keyDown(slider, { key: 'ArrowRight' })
    expect(onSeek).toHaveBeenLastCalledWith(35)
    fireEvent.keyDown(slider, { key: 'ArrowLeft' })
    expect(onSeek).toHaveBeenLastCalledWith(25)
  })

  it('jumps to start and end with Home/End', () => {
    const onSeek = vi.fn()
    render(<SeekBar position={30} duration={120} onSeek={onSeek} />)
    const slider = screen.getByRole('slider', { name: 'Seek' })
    fireEvent.keyDown(slider, { key: 'Home' })
    expect(onSeek).toHaveBeenLastCalledWith(0)
    fireEvent.keyDown(slider, { key: 'End' })
    expect(onSeek).toHaveBeenLastCalledWith(120)
  })

  it('clamps arrow-key seeks to the track bounds', () => {
    const onSeek = vi.fn()
    const { rerender } = render(<SeekBar position={2} duration={10} onSeek={onSeek} />)
    const slider = screen.getByRole('slider', { name: 'Seek' })
    fireEvent.keyDown(slider, { key: 'ArrowLeft' })
    expect(onSeek).toHaveBeenLastCalledWith(0)
    rerender(<SeekBar position={8} duration={10} onSeek={onSeek} />)
    fireEvent.keyDown(slider, { key: 'ArrowRight' })
    expect(onSeek).toHaveBeenLastCalledWith(10)
  })

  it('ignores seeks before the track duration is known', () => {
    const onSeek = vi.fn()
    render(<SeekBar position={0} duration={0} onSeek={onSeek} />)
    fireEvent.keyDown(screen.getByRole('slider', { name: 'Seek' }), { key: 'ArrowRight' })
    expect(onSeek).not.toHaveBeenCalled()
  })
})
