import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { ReactionBar, ReactionsOverlay, REACTIONS, type FloatingReaction } from '../components/Reactions'

describe('ReactionBar', () => {
  it('renders a button for every reaction', () => {
    render(<ReactionBar onReact={vi.fn()} />)
    expect(screen.getAllByRole('button')).toHaveLength(REACTIONS.length)
  })

  it('calls onReact with the clicked emoji', () => {
    const onReact = vi.fn()
    render(<ReactionBar onReact={onReact} />)
    fireEvent.click(screen.getByLabelText(`React ${REACTIONS[0]}`))
    expect(onReact).toHaveBeenCalledWith(REACTIONS[0])
  })
})

describe('ReactionsOverlay', () => {
  const item = (over: Partial<FloatingReaction> = {}): FloatingReaction => ({
    key: 'k1', emoji: '🔥', username: 'Alice', left: 40, ...over,
  })

  it('renders nothing when empty', () => {
    const { container } = render(<ReactionsOverlay items={[]} />)
    expect(container.querySelectorAll('.floating-reaction')).toHaveLength(0)
  })

  it('renders each floating reaction with its emoji and username', () => {
    render(<ReactionsOverlay items={[item(), item({ key: 'k2', emoji: '🎉', username: 'Bob' })]} />)
    expect(screen.getByText('🔥')).toBeInTheDocument()
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })

  it('positions a reaction horizontally from its left value', () => {
    const { container } = render(<ReactionsOverlay items={[item({ left: 63 })]} />)
    expect(container.querySelector('.floating-reaction')).toHaveStyle({ left: '63%' })
  })
})
