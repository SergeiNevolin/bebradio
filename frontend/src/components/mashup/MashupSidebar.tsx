import { useMemo, useState } from 'react'
import type { Mashup } from '../../types'
import { formatTime } from '../../lib/format'
import { monoGlyph, tintForId } from '../../lib/mashupArt'
import styles from './MashupSidebar.module.css'

type Filter = 'all' | 'liked' | 'mine'

interface MashupSidebarProps {
  items: Mashup[]
  likedItems: Mashup[]
  mineItems: Mashup[]
  signedIn: boolean
  activeId?: string
  isPlaying: boolean
  loading: boolean
  onPlay: (m: Mashup) => void
}

export default function MashupSidebar({
  items,
  likedItems,
  mineItems,
  signedIn,
  activeId,
  isPlaying,
  loading,
  onPlay,
}: MashupSidebarProps) {
  const [filter, setFilter] = useState<Filter>('all')
  const effective: Filter = filter !== 'all' && !signedIn ? 'all' : filter

  const list = useMemo(() => {
    if (effective === 'liked') return likedItems
    if (effective === 'mine') return mineItems
    return items
  }, [effective, items, likedItems, mineItems])

  const chips: { key: Filter; label: string }[] = [
    { key: 'all', label: 'All' },
    ...(signedIn
      ? ([
          { key: 'liked', label: 'Liked' },
          { key: 'mine', label: 'Yours' },
        ] as { key: Filter; label: string }[])
      : []),
  ]

  return (
    <div className={styles.sidebar}>
      <div className={styles.library}>
        <div className={styles.head}>
          <h2>Your library</h2>
        </div>

        <div className={styles.chips}>
          {chips.map((c) => (
            <button
              key={c.key}
              type="button"
              className={`${styles.chip} ${effective === c.key ? styles.chipOn : ''}`}
              onClick={() => setFilter(c.key)}
            >
              {c.label}
            </button>
          ))}
        </div>

        <div className={styles.sortRow}>
          <span>{loading ? '' : `${list.length} ${list.length === 1 ? 'item' : 'items'}`}</span>
          <span className={styles.sortBtn}>Recents ▾</span>
        </div>

        <div className={styles.scroll}>
          {loading ? (
            Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className={styles.skRow}>
                <div className={`${styles.sk} ${styles.skArt}`} />
                <div className={styles.skBody}>
                  <div className={`${styles.sk} ${styles.skLine}`} style={{ width: `${70 - i * 4}%` }} />
                  <div className={`${styles.sk} ${styles.skLine}`} style={{ width: `${45 - i * 3}%` }} />
                </div>
              </div>
            ))
          ) : list.length === 0 ? (
            <p className={styles.empty}>
              {effective === 'liked'
                ? 'No liked mashups yet.'
                : effective === 'mine'
                  ? 'You have not uploaded anything yet.'
                  : 'No mashups yet.'}
            </p>
          ) : (
            list.map((m) => {
              const active = m.id === activeId
              const ready = m.status === 'ready'
              return (
                <button
                  key={m.id}
                  type="button"
                  className={`${styles.row} ${active ? styles.rowOn : ''}`}
                  onClick={() => onPlay(m)}
                  disabled={!ready}
                >
                  {m.cover_url ? (
                    <img className={styles.rowArt} src={m.cover_url} alt="" />
                  ) : (
                    <span className={styles.rowArt} style={{ background: tintForId(m.id) }}>
                      <span className={styles.glyph} aria-hidden="true">{monoGlyph(m.title)}</span>
                    </span>
                  )}
                  {active && isPlaying && (
                    <span className={styles.eq} aria-hidden="true"><i /><i /><i /></span>
                  )}
                  <span className={styles.rowBody}>
                    <span className={styles.rowTitle} title={m.title}>{m.title}</span>
                    <span className={styles.rowMeta}>Mashup · {m.owner_name || 'unknown'}</span>
                  </span>
                  {ready && <span className={styles.rowDur}>{formatTime(m.duration)}</span>}
                </button>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}
