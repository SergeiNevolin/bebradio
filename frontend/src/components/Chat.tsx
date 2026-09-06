import { useState, useRef, useEffect, memo } from 'react'
import styles from './Chat.module.css'
import Avatar from './Avatar'
import { useUserAvatars } from '../hooks/useUserAvatars'

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
  // Opens the user's profile in a modal (provided by the room page). When
  // omitted, usernames render as plain text.
  onSelectUser?: (userId: string) => void
}

function Chat({ messages, onSend, currentUserId, onSelectUser }: ChatProps) {
  const [text, setText] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const prevLen = useRef(messages.length)
  const avatars = useUserAvatars(messages.map((m) => m.user_id))

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
        {messages.map((msg) => {
          const own = msg.user_id === currentUserId
          const clickable = !!msg.user_id && !!onSelectUser
          const avatar = <Avatar name={msg.username} src={avatars[msg.user_id]} size={28} />
          return (
            <div
              key={msg.id}
              className={`${styles.chatMessage} ${own ? styles.chatMessageOwn : ''}`}
            >
              <div className={styles.chatRow}>
                {clickable ? (
                  <button
                    type="button"
                    className={styles.chatAvatarBtn}
                    onClick={() => onSelectUser!(msg.user_id)}
                    aria-label={`Open ${msg.username}'s profile`}
                  >
                    {avatar}
                  </button>
                ) : (
                  avatar
                )}
                <div className={styles.chatBody}>
                  {clickable ? (
                    <button
                      type="button"
                      className={`${styles.chatUsername} profile-link`}
                      onClick={() => onSelectUser!(msg.user_id)}
                    >
                      {msg.username}
                    </button>
                  ) : (
                    <span className={styles.chatUsername}>{msg.username}</span>
                  )}
                  <span className={styles.chatText}>{msg.text}</span>
                </div>
              </div>
            </div>
          )
        })}
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
