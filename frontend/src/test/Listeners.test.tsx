import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import Listeners from '../components/Listeners'

const registered = [
  { id: 'u-charlie', name: 'Charlie' },
  { id: 'u-alice', name: 'alice' },
  { id: 'u-bob', name: 'Bob' },
]

beforeEach(() => {
  localStorage.clear()
})

describe('Listeners', () => {
  it('shows the total count in the header', () => {
    render(<Listeners listeners={registered} ownerId="" onSelectUser={() => {}} />)
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('sorts registered listeners alphabetically, case-insensitively', () => {
    const { container } = render(
      <Listeners listeners={registered} ownerId="" onSelectUser={() => {}} />,
    )
    const names = Array.from(container.querySelectorAll('.name')).map((el) => el.textContent)
    expect(names).toEqual(['alice', 'Bob', 'Charlie'])
  })

  it('marks the room creator with a crown', () => {
    render(<Listeners listeners={registered} ownerId="u-bob" onSelectUser={() => {}} />)
    const bobRow = screen.getByText('Bob').closest('button')!
    expect(bobRow.textContent).toContain('👑')
    const aliceRow = screen.getByText('alice').closest('button')!
    expect(aliceRow.textContent).not.toContain('👑')
  })

  it('aggregates anonymous listeners into a guest count', () => {
    const listeners = [
      ...registered,
      { id: 'anon:1.2.3.4:5000', name: 'Anonymous' },
      { id: 'anon:5.6.7.8:6000', name: 'Anonymous' },
    ]
    render(<Listeners listeners={listeners} ownerId="" onSelectUser={() => {}} />)
    expect(screen.getByText('+ 2 guests')).toBeInTheDocument()
    expect(screen.queryByText('Anonymous')).not.toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('uses the singular form for a single guest', () => {
    render(
      <Listeners
        listeners={[{ id: 'anon:1.2.3.4:5000', name: 'Anonymous' }]}
        ownerId=""
        onSelectUser={() => {}}
      />,
    )
    expect(screen.getByText('+ 1 guest')).toBeInTheDocument()
  })

  it('calls onSelectUser with the user id when a listener is clicked', () => {
    const onSelectUser = vi.fn()
    render(<Listeners listeners={registered} ownerId="" onSelectUser={onSelectUser} />)
    fireEvent.click(screen.getByText('Bob'))
    expect(onSelectUser).toHaveBeenCalledWith('u-bob')
  })

  it('hides the list when collapsed and persists the choice', () => {
    const { unmount } = render(
      <Listeners listeners={registered} ownerId="" onSelectUser={() => {}} />,
    )
    expect(screen.getByText('Bob')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /hide listeners/i }))
    expect(screen.queryByText('Bob')).not.toBeInTheDocument()
    expect(localStorage.getItem('listeners-collapsed')).toBe('1')

    unmount()
    render(<Listeners listeners={registered} ownerId="" onSelectUser={() => {}} />)
    expect(screen.queryByText('Bob')).not.toBeInTheDocument()
  })
})
