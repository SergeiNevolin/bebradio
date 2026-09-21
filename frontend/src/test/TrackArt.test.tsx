import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import TrackArt from '../components/TrackArt'

describe('TrackArt', () => {
  it('renders an img when a thumbnail exists', () => {
    const { container } = render(
      <TrackArt id="t1" title="Song" thumbnail="https://example.com/a.jpg" size={48} className="slot" />,
    )
    const img = container.querySelector('img.slot') as HTMLImageElement
    expect(img).toBeInTheDocument()
    expect(img.src).toBe('https://example.com/a.jpg')
  })

  it('renders a title glyph placeholder when the artwork is missing', () => {
    const { container } = render(
      <TrackArt id="t1" title="bootleg" size={48} className="slot" />,
    )
    expect(container.querySelector('img')).not.toBeInTheDocument()
    const fallback = container.querySelector('.slot') as HTMLElement
    expect(fallback).toBeInTheDocument()
    expect(fallback).toHaveTextContent('B')
    expect(fallback.style.background).toBeTruthy()
  })

  it('falls back to M for an empty title', () => {
    render(<TrackArt id="t1" title="" size={48} />)
    expect(screen.getByText('M')).toBeInTheDocument()
  })

  it('honours explicit height for rectangular slots', () => {
    const { container } = render(
      <TrackArt id="t1" title="Song" size={48} height={36} />,
    )
    const fallback = container.firstChild as HTMLElement
    expect(fallback.style.height).toBe('36px')
    expect(fallback.style.width).toBe('48px')
  })
})
