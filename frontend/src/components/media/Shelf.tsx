import type { MutableRefObject, ReactNode } from 'react'
import { Link } from 'react-router-dom'
import ScrollRow from '../ScrollRow'
import styles from './Shelf.module.css'

interface ShelfProps {
  /** Section title (h2). */
  title: string
  /** Muted counter next to the title, e.g. track count. */
  count?: ReactNode
  /** "Show all" link target. Default label — «Показать все». */
  linkTo?: string
  linkLabel?: string
  /** Extra header actions (buttons) rendered right of the link. */
  actions?: ReactNode
  /** Cards / rows inside the horizontal rail. */
  children: ReactNode
  /** Shown instead of the rail while loading (shimmer cards). */
  loading?: boolean
  skeletonCount?: number
  /** Shown instead of the rail when there is nothing to display. */
  empty?: ReactNode
  /** Forwarded to the rail's scroll box (e.g. infinite-scroll sentinel root). */
  trackRef?: MutableRefObject<HTMLDivElement | null> | ((el: HTMLDivElement | null) => void)
  /** Trailing node inside the rail (e.g. an intersection sentinel). */
  sentinel?: ReactNode
  /** Compact secondary header (14px muted) instead of the section title. */
  small?: boolean
}

/**
 * Единая горизонтальная полка bebradio: заголовок раздела (стиль Home) +
 * горизонтальный скролл карточек. Используется на главной, Rooms и Mashups.
 */
export default function Shelf({
  title,
  count,
  linkTo,
  linkLabel = 'Показать все',
  actions,
  children,
  loading = false,
  skeletonCount = 5,
  empty,
  trackRef,
  sentinel,
  small = false,
}: ShelfProps) {
  return (
    <section className={styles.shelf}>
      <div className={`${styles.header}${small ? ` ${styles.headerSmall}` : ''}`}>
        {linkTo ? (
          <Link to={linkTo} className={styles.titleLink} aria-label={`${title} — ${linkLabel}`}>
            <h2 className={small ? styles.titleSmall : styles.title}>{title}</h2>
          </Link>
        ) : (
          <h2 className={small ? styles.titleSmall : styles.title}>{title}</h2>
        )}
        <div className={styles.headerRight}>
          {count !== undefined && <span className={styles.count}>{count}</span>}
          {linkTo && (
            <Link className={styles.link} to={linkTo}>
              {linkLabel}
            </Link>
          )}
          {actions}
        </div>
      </div>
      {loading ? (
        <div className={styles.skeleton} aria-label={`Загрузка: ${title}`}>
          {Array.from({ length: skeletonCount }).map((_, i) => (
            <div key={i} className={styles.skeletonCard} />
          ))}
        </div>
      ) : empty ? (
        <div className={styles.emptyWrap}>{empty}</div>
      ) : (
        <ScrollRow trackRef={trackRef}>
          {children}
          {sentinel}
        </ScrollRow>
      )}
    </section>
  )
}
