import { memo, useState } from 'react'
import type { Track } from '../types'
import TrackArt from './TrackArt'
import styles from './Queue.module.css'

interface QueueProps {
  queue: Track[]
  currentIndex: number
  /** Auto-radio is fetching related tracks right now. */
  searching?: boolean
  /** Admin-only: offer "Save to bebradio" on YouTube tracks. */
  canImport?: boolean
  onImport?: (trackId: string) => Promise<void>
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

function Queue({ queue, currentIndex, searching = false, canImport = false, onImport }: QueueProps) {
  const [importingId, setImportingId] = useState<string | null>(null)

  const handleImport = async (trackId: string) => {
    if (!onImport || importingId !== null) return
    setImportingId(trackId)
    try {
      await onImport(trackId)
    } finally {
      setImportingId(null)
    }
  };
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
              <TrackArt
                id={track.id}
                title={track.title}
                thumbnail={track.thumbnail}
                size={48}
                radius={6}
                className={styles.queueItemThumb}
              />
              <div className={styles.queueItemInfo}>
                <div className={styles.title}>{track.title}</div>
                <div className={styles.artist}>{track.artist || 'Unknown artist'}</div>
              </div>
              {badge && <span className={styles.sourceBadge}>{badge}</span>}
              {canImport && onImport && track.source === 'youtube' && (
                <button
                  type="button"
                  className={styles.importBtn}
                  disabled={importingId !== null}
                  onClick={() => handleImport(track.id)}
                  title="Save track to the bebradio library"
                >
                  {importingId === track.id ? 'Saving…' : 'Save to bebradio'}
                </button>
              )}
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
