import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import Queue from '../components/Queue'
import type { Track } from '../types'

const tracks: Track[] = [
  { id: '1', title: 'Song A', artist: 'Artist A', thumbnail: '', url: '', duration: 0, added_by: '' },
  { id: '2', title: 'Song B', artist: 'Artist B', thumbnail: '', url: '', duration: 0, added_by: '' },
  { id: '3', title: 'Song C', artist: 'Artist C', thumbnail: '', url: '', duration: 0, added_by: '' },
]

describe('Queue', () => {
  it('shows empty message when no tracks', () => {
    render(<Queue queue={[]} currentIndex={0} />)
    expect(screen.getByText('No tracks yet. Add a YouTube link above.')).toBeInTheDocument()
  })

  it('renders track list', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('Song A')).toBeInTheDocument()
    expect(screen.getByText('Song B')).toBeInTheDocument()
    expect(screen.getByText('Song C')).toBeInTheDocument()
  })

  it('shows queue count', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('Queue (3)')).toBeInTheDocument()
  })

  it('highlights current track', () => {
    const { container } = render(<Queue queue={tracks} currentIndex={1} />)
    const items = container.querySelectorAll('.queue-item')
    expect(items[1]).toHaveClass('active')
    expect(items[0]).not.toHaveClass('active')
  })

  it('renders track numbers', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('renders artist names', () => {
    render(<Queue queue={tracks} currentIndex={0} />)
    expect(screen.getByText('Artist A')).toBeInTheDocument()
    expect(screen.getByText('Artist B')).toBeInTheDocument()
  })
})
