import { memo, useCallback, useState } from 'react'
import styles from './Listeners.module.css'

interface Listener {
  id: string
  name: string
}

interface ListenersProps {
  listeners: Listener[]
  ownerId: string
  onSelectUser: (userId: string) => void
}

const STORAGE_KEY = 'listeners-collapsed'

// Anonymous listeners get a synthetic "anon:<addr>" id from the backend and have
// no real profile to open, so they are aggregated into a single guest count.
const isAnon = (id: string) => id.startsWith('anon:')

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

function Listeners({ listeners, ownerId, onSelectUser }: ListenersProps) {
  const [collapsed, setCollapsed] = useState(readCollapsed)

  const toggle = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev
      try {
        localStorage.setItem(STORAGE_KEY, next ? '1' : '0')
      } catch { /* ignore */ }
      return next
    })
  }, [])

  const registered = listeners
    .filter((l) => !isAnon(l.id))
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
  const guestCount = listeners.length - registered.length
  const total = listeners.length

  return (
    <aside className={styles.panel} data-collapsed={collapsed}>
      <button
        className={styles.header}
        onClick={toggle}
        aria-expanded={!collapsed}
        aria-label={collapsed ? 'Show listeners' : 'Hide listeners'}
      >
        <span className={styles.title}>Listeners</span>
        <span className={styles.count}>{total}</span>
        <span className={styles.chevron} aria-hidden="true">{collapsed ? '▸' : '▾'}</span>
      </button>

      {!collapsed && (
        <div className={styles.body}>
          {total === 0 && <div className={styles.empty}>No one here yet</div>}
          {registered.length > 0 && (
            <ul className={styles.list}>
              {registered.map((l) => (
                <li key={l.id}>
                  <button className={styles.row} onClick={() => onSelectUser(l.id)}>
                    <span className={styles.name}>{l.name}</span>
                    {l.id === ownerId && (
                      <span className={styles.crown} title="Room creator">👑</span>
                    )}
                  </button>
                </li>
              ))}
            </ul>
          )}
          {guestCount > 0 && (
            <div className={styles.guests}>
              + {guestCount} {guestCount === 1 ? 'guest' : 'guests'}
            </div>
          )}
        </div>
      )}
    </aside>
  )
}

export default memo(Listeners)
