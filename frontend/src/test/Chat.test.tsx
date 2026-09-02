import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Chat from '../components/Chat'

const messages = [
  { id: '1', user_id: 'u1', username: 'Alice', text: 'Hello!', created_at: 1 },
  { id: '2', user_id: 'u2', username: 'Bob', text: 'Hey there', created_at: 2 },
]

describe('Chat', () => {
  it('renders chat with messages', () => {
    render(<MemoryRouter><Chat messages={messages} onSend={vi.fn()} /></MemoryRouter>)
    expect(screen.getByText('Hello!')).toBeInTheDocument()
    expect(screen.getByText('Hey there')).toBeInTheDocument()
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })

  it('renders empty state', () => {
    render(<MemoryRouter><Chat messages={[]} onSend={vi.fn()} /></MemoryRouter>)
    expect(screen.getByText('No messages yet')).toBeInTheDocument()
  })

  it('calls onSend with text on submit', () => {
    const onSend = vi.fn()
    render(<MemoryRouter><Chat messages={[]} onSend={onSend} /></MemoryRouter>)
    const input = screen.getByPlaceholderText('Type a message...')
    fireEvent.change(input, { target: { value: 'My message' } })
    fireEvent.click(screen.getByText('Send'))
    expect(onSend).toHaveBeenCalledWith('My message')
  })

  it('clears input after send', () => {
    render(<MemoryRouter><Chat messages={[]} onSend={vi.fn()} /></MemoryRouter>)
    const input = screen.getByPlaceholderText('Type a message...')
    fireEvent.change(input, { target: { value: 'Test' } })
    fireEvent.click(screen.getByText('Send'))
    expect(input).toHaveValue('')
  })

  it('does not send empty message', () => {
    const onSend = vi.fn()
    render(<MemoryRouter><Chat messages={[]} onSend={onSend} /></MemoryRouter>)
    fireEvent.click(screen.getByText('Send'))
    expect(onSend).not.toHaveBeenCalled()
  })

  it('highlights own messages', () => {
    render(
      <MemoryRouter>
        <Chat
          messages={[{ id: '1', user_id: 'me', username: 'Me', text: 'Mine', created_at: 1 }]}
          onSend={vi.fn()}
          currentUserId="me"
        />
      </MemoryRouter>
    )
    expect(screen.getByText('Mine').closest('.chat-message')).toHaveClass('chat-message-own')
  })
})
