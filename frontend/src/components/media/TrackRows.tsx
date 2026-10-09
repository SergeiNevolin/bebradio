import type { CSSProperties, ReactNode } from 'react'
import { PlayIcon } from '../player/icons'
import styles from './TrackRows.module.css'

interface TrackRowProps {
  title: string
  /** Вторая строка: «N строк · LANG», исполнитель и т.п. */
  meta?: ReactNode
  /** Пилюля-бейдж в строке меты («свой текст», источник). */
  badge?: ReactNode
  /** Строка ошибки под метой. */
  error?: string | null
  /** Порядковый номер слева (плейлистный вид). */
  number?: ReactNode
  /** Длительность справа приглушённой (плейлистный вид). */
  duration?: string
  artUrl?: string
  /** Фон-заглушка обложки. */
  tint?: string
  /** Иконка-заглушка обложки. */
  glyph?: ReactNode
  /** Спиннер вместо обложки (песня грузится). */
  busy?: boolean
  onPlay: () => void
  /** aria-label кнопок проигрывания. */
  playLabel: string
  /** Неготово к игре (processing/failed): кнопки молчат, как у карточек. */
  disabled?: boolean
  /** aria-label в disabled-режиме. По умолчанию — playLabel. */
  disabledLabel?: string
  /** Правые контролы строки (лайк, «Петь», длительность). */
  actions?: ReactNode
  /** Порядок в списке — лёгкий stagger появления. */
  index?: number
}

/**
 * Единая строка вертикального списка треков: обложка 44 с hover-play,
 * название + мета, правые контролы. Словарь — строки очереди/сайдбара.
 */
export function TrackRow({
  title,
  meta,
  badge,
  error,
  number,
  duration,
  artUrl,
  tint,
  glyph,
  busy = false,
  onPlay,
  playLabel,
  disabled = false,
  disabledLabel,
  actions,
  index = 0,
}: TrackRowProps) {
  const stagger: CSSProperties =
    index > 0 ? { animationDelay: `${Math.min(index * 0.04, 0.35)}s` } : {}
  return (
    <div className={`group ${styles.row}${disabled ? ` ${styles.rowDisabled}` : ''}`} style={stagger}>
      {number !== undefined && (
        <span className={styles.num} aria-hidden="true">
          {number}
        </span>
      )}
      <button
        type="button"
        onClick={onPlay}
        aria-label={disabled ? (disabledLabel ?? playLabel) : playLabel}
        className={styles.art}
        style={artUrl ? undefined : { background: tint }}
        disabled={disabled}
      >
        {busy ? (
          <span className={styles.spinner} aria-hidden="true" />
        ) : (
          <>
            {artUrl ? (
              <img className={styles.img} src={artUrl} alt="" />
            ) : (
              <span className={styles.glyph} aria-hidden="true">
                {glyph}
              </span>
            )}
            <span aria-hidden="true" className={styles.artHover}>
              <span className={styles.artPlay}>
                <PlayIcon size={14} />
              </span>
            </span>
          </>
        )}
      </button>
      <button type="button" onClick={disabled ? undefined : onPlay} className={styles.body}>
        <div className={styles.title}>{title}</div>
        {(meta !== undefined || badge) && (
          <div className={styles.meta}>
            {meta && <span className={styles.metaText}>{meta}</span>}
            {badge}
          </div>
        )}
        {error && <span className={styles.error}>{error}</span>}
      </button>
      {duration && (
        <span className={styles.dur} title={duration}>
          {duration}
        </span>
      )}
      {actions}
    </div>
  )
}

/** Единая панель вертикального списка (поверх строк). */
export function TrackList({ children }: { children: ReactNode }) {
  return <div className={styles.panel}>{children}</div>
}
