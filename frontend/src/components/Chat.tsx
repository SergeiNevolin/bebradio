import { useState, useRef, useEffect } from 'react'
import { Link } from 'react-router-dom'

export interface ChatMessage {
  id: string
  user_id: string
  username: string
  text: string
  created_at: number
}

interface ChatProps {
  messages: ChatMessage[]
  onSend: (text: string) => void
  currentUserId?: string
}

export default function Chat({ messages, onSend, currentUserId }: ChatProps) {
  const [text, setText] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const prevLen = useRef(messages.length)

  useEffect(() => {
    // Only auto-scroll when a *new* message arrives, not on initial load.
    if (messages.length > prevLen.current && bottomRef.current?.scrollIntoView) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' })
    }
    prevLen.current = messages.length
  }, [messages.length])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = text.trim()
    if (!trimmed) return
    onSend(trimmed)
    setText('')
  }

  return (
    <div className="chat">
      <div className="chat-header">Chat</div>
      <div className="chat-messages">
        {messages.length === 0 && (
          <div className="chat-empty">No messages yet</div>
        )}
        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`chat-message ${msg.user_id === currentUserId ? 'chat-message-own' : ''}`}
          >
            {msg.user_id ? (
              <Link to={`/user/${msg.user_id}`} className="chat-username profile-link">
                {msg.username}
              </Link>
            ) : (
              <span className="chat-username">{msg.username}</span>
            )}
            <span className="chat-text">{msg.text}</span>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
      <form className="chat-input" onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="Type a message..."
          value={text}
          onChange={(e) => setText(e.target.value)}
          maxLength={500}
        />
        <button type="submit" className="btn btn-sm" disabled={!text.trim()}>
          Send
        </button>
      </form>
    </div>
  )
}
