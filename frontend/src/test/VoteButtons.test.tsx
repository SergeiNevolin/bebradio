import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import VoteButtons from '../components/VoteButtons'

describe('VoteButtons', () => {
  const defaultProps = {
    trackId: 't1',
    likes: 3,
    dislikes: 1,
    userVote: 0,
    skipVoters: [],
    currentUserId: 'u1',
    onVote: vi.fn(),
    onSkipVote: vi.fn(),
  }

  it('renders like and dislike counts', () => {
    render(<VoteButtons {...defaultProps} />)
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()
  })

  it('calls onVote with 1 when like clicked', () => {
    const onVote = vi.fn()
    render(<VoteButtons {...defaultProps} onVote={onVote} />)
    fireEvent.click(screen.getByTitle('Like'))
    expect(onVote).toHaveBeenCalledWith('t1', 1)
  })

  it('calls onVote with 0 when like clicked again (toggle)', () => {
    const onVote = vi.fn()
    render(<VoteButtons {...defaultProps} userVote={1} onVote={onVote} />)
    fireEvent.click(screen.getByTitle('Like'))
    expect(onVote).toHaveBeenCalledWith('t1', 0)
  })

  it('calls onVote with -1 when dislike clicked', () => {
    const onVote = vi.fn()
    render(<VoteButtons {...defaultProps} onVote={onVote} />)
    fireEvent.click(screen.getByTitle('Dislike'))
    expect(onVote).toHaveBeenCalledWith('t1', -1)
  })

  it('shows skip vote count', () => {
    render(<VoteButtons {...defaultProps} skipVoters={['u1', 'u2']} />)
    expect(screen.getByText('Skip (2)')).toBeInTheDocument()
  })

  it('calls onSkipVote when skip clicked', () => {
    const onSkipVote = vi.fn()
    render(<VoteButtons {...defaultProps} onSkipVote={onSkipVote} />)
    fireEvent.click(screen.getByTitle('Vote to skip'))
    expect(onSkipVote).toHaveBeenCalled()
  })

  it('highlights active like vote', () => {
    render(<VoteButtons {...defaultProps} userVote={1} />)
    expect(screen.getByTitle('Like').closest('.vote-btn')).toHaveClass('vote-btn-active')
  })

  it('highlights active dislike vote', () => {
    render(<VoteButtons {...defaultProps} userVote={-1} />)
    expect(screen.getByTitle('Dislike').closest('.vote-btn')).toHaveClass('vote-btn-active-down')
  })

  it('highlights active skip vote', () => {
    render(<VoteButtons {...defaultProps} skipVoters={['u1']} />)
    expect(screen.getByTitle('Vote to skip').closest('.vote-btn')).toHaveClass('vote-skip-active')
  })
})
