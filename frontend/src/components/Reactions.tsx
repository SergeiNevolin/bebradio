// Keep in sync with REACTION_EMOJIS on the backend (config.py).
import styles from './Reactions.module.css'

export const REACTIONS = ['❤️', '🔥', '😂', '👍', '🎉', '😮', '🙌', '💃']

export interface FloatingReaction {
  key: string
  emoji: string
  username: string
  left: number
}

export function ReactionBar({ onReact }: { onReact: (emoji: string) => void }) {
  return (
    <div className={styles.reactionBar}>
      {REACTIONS.map((emoji) => (
        <button
          key={emoji}
          type="button"
          className={styles.reactionBtn}
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
    <div className={styles.reactionsOverlay} aria-hidden="true">
      {items.map((r) => (
        <span key={r.key} className={styles.floatingReaction} style={{ left: `${r.left}%` }}>
          <span className={styles.floatingReactionEmoji}>{r.emoji}</span>
          <span className={styles.floatingReactionUser}>{r.username}</span>
        </span>
      ))}
    </div>
  )
}
