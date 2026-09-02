import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import Listeners from '../components/Listeners'

describe('Listeners', () => {
  it('shows the listener count', () => {
    render(<Listeners listeners={[]} count={3} />)
    expect(screen.getByText('3 listening')).toBeInTheDocument()
  })

  it('renders a chip per listener name', () => {
    render(<Listeners listeners={[{ id: 'a', name: 'Alice' }, { id: 'b', name: 'Bob' }]} count={2} />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })

  it('collapses overflow into a +N chip', () => {
    const listeners = Array.from({ length: 7 }, (_, i) => ({ id: `u${i}`, name: `User${i}` }))
    render(<Listeners listeners={listeners} count={7} max={4} />)
    expect(screen.getByText('User3')).toBeInTheDocument()
    expect(screen.queryByText('User4')).not.toBeInTheDocument()
    expect(screen.getByText('+3')).toBeInTheDocument()
  })
})
