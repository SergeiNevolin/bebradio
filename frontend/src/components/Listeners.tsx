import { memo } from 'react'
import styles from './Listeners.module.css'

interface Listener {
  id: string
  name: string
}

interface ListenersProps {
  listeners: Listener[]
  count: number
  max?: number
}

function Listeners({ listeners, count, max = 4 }: ListenersProps) {
  const shown = listeners.slice(0, max)
  const extra = count - shown.length

  return (
    <span
      className={styles.listeners}
      title={listeners.length ? listeners.map((l) => l.name).join(', ') : undefined}
    >
      <span className="status-dot" />
      <span className={styles.listenersCount}>{count} listening</span>
      {shown.length > 0 && (
        <span className={styles.listenerChips}>
          {shown.map((l) => (
            <span key={l.id} className={styles.listenerChip}>{l.name}</span>
          ))}
          {extra > 0 && <span className={`${styles.listenerChip} ${styles.listenerChipMore}`}>+{extra}</span>}
        </span>
      )}
    </span>
  )
}

export default memo(Listeners)
