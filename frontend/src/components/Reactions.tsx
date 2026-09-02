// Keep in sync with REACTION_EMOJIS on the backend (config.py).
export const REACTIONS = ['❤️', '🔥', '😂', '👍', '🎉', '😮', '🙌', '💃']

export interface FloatingReaction {
  key: string
  emoji: string
  username: string
  left: number
}

export function ReactionBar({ onReact }: { onReact: (emoji: string) => void }) {
  return (
    <div className="reaction-bar">
      {REACTIONS.map((emoji) => (
        <button
          key={emoji}
          type="button"
          className="reaction-btn"
          onClick={() => onReact(emoji)}
          aria-label={`React ${emoji}`}
        >
          {emoji}
        </button>
      ))}
    </div>
  )
}

export function ReactionsOverlay({ items }: { items: FloatingReaction[] }) {
  return (
    <div className="reactions-overlay" aria-hidden="true">
      {items.map((r) => (
        <span key={r.key} className="floating-reaction" style={{ left: `${r.left}%` }}>
          <span className="floating-reaction-emoji">{r.emoji}</span>
          <span className="floating-reaction-user">{r.username}</span>
        </span>
      ))}
    </div>
  )
}
