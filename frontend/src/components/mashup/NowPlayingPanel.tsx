import type { Mashup } from '../../types'
import { formatTime } from '../../lib/format'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import styles from './NowPlayingPanel.module.css'

interface NowPlayingPanelProps {
  current: Mashup | null
  queue: Mashup[]
  loading: boolean
  onPlayFromQueue: (m: Mashup) => void
  onToggleLike: (m: Mashup) => void
  onOpenProfile?: (userId: string) => void
}

function Art({ mashup, className }: { mashup: Mashup; className: string }) {
  if (mashup.cover_url) {
    return <img className={className} src={mashup.cover_url} alt="" />
  }
  return (
    <div className={className} style={{ background: tintForId(mashup.id) }}>
      <span className={styles.glyph} aria-hidden="true">{monoGlyph(mashup.title)}</span>
    </div>
  )
}

export default function NowPlayingPanel({
  current,
  queue,
  loading,
  onPlayFromQueue,
  onToggleLike,
  onOpenProfile,
}: NowPlayingPanelProps) {
  return (
    <aside className={styles.panel} aria-label="Now playing">
      <div className={styles.head}>
        <h2>Now playing</h2>
      </div>

      <div className={styles.scroll}>
        {loading && (
          <div className={styles.bodyPad}>
            <div className={`${styles.skel} ${styles.skelArt}`} />
            <div className={`${styles.skel} ${styles.skelLine}`} style={{ width: '70%', marginTop: 14 }} />
            <div className={`${styles.skel} ${styles.skelLine}`} style={{ width: '45%', marginTop: 8 }} />
          </div>
        )}

        {!loading && !current && (
          <div className={styles.empty}>
            <div className={styles.emptyTitle}>Nothing playing</div>
            <div className={styles.emptySub}>Pick a mashup to start listening here.</div>
          </div>
        )}

        {!loading && current && (
          <div className={styles.bodyPad}>
            <Art mashup={current} className={styles.bigArt} />
            <div className={styles.bigTitle} title={current.title}>{current.title}</div>
            <div className={styles.bigMeta}>
              {current.artist || 'Unknown artist'}
              {current.owner_name && current.owner_id && onOpenProfile ? (
                <>
                  {' · by '}
                  <button
                    type="button"
                    className={styles.ownerLink}
                    onClick={() => onOpenProfile(current.owner_id)}
                  >
                    {current.owner_name}
                  </button>
                </>
              ) : current.owner_name ? (
                ` · by ${current.owner_name}`
              ) : null}
            </div>

            <div className={styles.row}>
              <button
                type="button"
                className={`${styles.likeBtn} ${current.liked ? styles.likeBtnOn : ''}`}
                onClick={() => onToggleLike(current)}
                aria-pressed={!!current.liked}
                aria-label={current.liked ? `Unlike ${current.title}` : `Like ${current.title}`}
              >
                <span className={styles.heart} aria-hidden="true">{current.liked ? '♥' : '♡'}</span>
                {current.likes}
              </button>
              {current.status === 'ready' && (
                <span className={styles.dur}>{formatTime(current.duration)}</span>
              )}
            </div>

            <h3 className={styles.queueHead}>Next in queue</h3>
            {queue.length === 0 ? (
              <p className={styles.queueEmpty}>End of the list.</p>
            ) : (
              queue.map((m) => (
                <button
                  key={m.id}
                  type="button"
                  className={styles.qrow}
                  onClick={() => onPlayFromQueue(m)}
                >
                  <Art mashup={m} className={styles.qart} />
                  <span className={styles.qbody}>
                    <span className={styles.qtitle} title={m.title}>{m.title}</span>
                    <span className={styles.qmeta}>{m.artist || 'Unknown artist'}</span>
                  </span>
                </button>
              ))
            )}
          </div>
        )}
      </div>
    </aside>
  )
}
