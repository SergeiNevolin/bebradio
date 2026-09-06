import { useState, useRef, useEffect, memo } from 'react'
import { Link } from 'react-router-dom'
import styles from './Chat.module.css'

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

function Chat({ messages, onSend, currentUserId }: ChatProps) {
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
    <div className={styles.chat}>
      <div className={styles.chatHeader}>Chat</div>
      <div className={styles.chatMessages}>
        {messages.length === 0 && (
          <div className={styles.chatEmpty}>No messages yet</div>
        )}
        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`${styles.chatMessage} ${msg.user_id === currentUserId ? styles.chatMessageOwn : ''}`}
          >
            {msg.user_id ? (
              <Link to={`/user/${msg.user_id}`} className={`${styles.chatUsername} profile-link`}>
                {msg.username}
              </Link>
            ) : (
              <span className={styles.chatUsername}>{msg.username}</span>
            )}
            <span className={styles.chatText}>{msg.text}</span>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
      <form className={styles.chatInput} onSubmit={handleSubmit}>
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

export default memo(Chat)
