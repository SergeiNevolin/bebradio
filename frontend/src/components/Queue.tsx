import { memo } from 'react'
import type { Track } from '../types'
import styles from './Queue.module.css'

interface QueueProps {
  queue: Track[]
  currentIndex: number
  /** Auto-radio is fetching related tracks right now. */
  searching?: boolean
}

// Human labels for known queue sources. Unknown (future) sources fall back
// to the raw string so new providers show up without UI changes.
const SOURCE_LABELS: Record<string, string> = {
  youtube: 'YouTube',
  upload: 'Mashup',
}

function sourceLabel(source: Track['source']): string | null {
  if (!source) return null
  return SOURCE_LABELS[source] ?? source
}

function Queue({ queue, currentIndex, searching = false }: QueueProps) {
  if (!queue.length) {
    return (
      <div className={styles.queue}>
        <h3>Queue</h3>
        <div className={styles.queueEmpty}>
          {searching ? (
            <span className={styles.queueSearching}>
              <span className={styles.radioSpinner} aria-hidden="true" />
              Radio is finding tracks…
            </span>
          ) : (
            'No tracks yet. Add something above.'
          )}
        </div>
      </div>
    )
  }

  return (
    <div className={styles.queue}>
      <h3>Queue ({queue.length})</h3>
      <div className={styles.queueList}>
        {queue.map((track, i) => {
          const badge = sourceLabel(track.source)
          return (
            <div
              key={track.id}
              className={`${styles.queueItem} ${i === currentIndex ? styles.queueItemActive : ''}`}
            >
              <span className={styles.queueItemNum}>{i + 1}</span>
              {track.thumbnail && (
                <img className={styles.queueItemThumb} src={track.thumbnail} alt="" />
              )}
              <div className={styles.queueItemInfo}>
                <div className={styles.title}>{track.title}</div>
                <div className={styles.artist}>{track.artist}</div>
              </div>
              {badge && <span className={styles.sourceBadge}>{badge}</span>}
            </div>
          )
        })}
      </div>
      {searching && (
        <div className={`${styles.queueSearching} ${styles.queueSearchingFoot}`}>
          <span className={styles.radioSpinner} aria-hidden="true" />
          Radio is finding more tracks…
        </div>
      )}
    </div>
  )
}

export default memo(Queue)
