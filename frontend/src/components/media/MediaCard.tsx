import type { ReactNode } from 'react'
import { PauseIcon, PlayIcon } from '../player/icons'
import styles from './MediaCard.module.css'

interface MediaCardProps {
  title: string
  /** Вторая строка: исполнитель, язык · длительность и т.п. */
  meta?: ReactNode
  /** Тултип строки меты. */
  metaTitle?: string
  coverUrl?: string
  /** Фон-заглушка без обложки (tintForId). */
  tint?: string
  /** Буква-заглушка без обложки. */
  glyph?: ReactNode
  active?: boolean
  playing?: boolean
  /** Показывать эквалайзер поверх обложки во время игры (стиль мэшапов). */
  showEq?: boolean
  /** Квадратная обложка трека или круглый аватар (полка исполнителей). */
  shape?: 'square' | 'round'
  /** Показывать hover-FAB (у аватаров исполнителей он врёт — там не play). */
  showFab?: boolean
  /** aria-label кнопки play/pause. */
  playLabel: string
  onToggle: () => void
  /** Неготово к игре (processing/failed): клик и FAB недоступны. */
  disabled?: boolean
  /** Нижний ряд для страничного экшена (лайк, длительность, edit). */
  foot?: ReactNode
  testId?: string
  dataTrackId?: string
  pressed?: boolean
  /**
   * true — вся карточка одна кнопка (полки Home);
   * false — div + внутренняя кнопка обложки (есть вложенные кнопки в foot).
   */
  asButton?: boolean
  /** md — крупные 176px, sm — стандарт 144px для всех полок. */
  size?: 'md' | 'sm'
}

/**
 * Единая карточка трека bebradio: квадратная обложка, hover-FAB play/pause,
 * название + мета. Основа — язык полок главной (Home).
 */
export default function MediaCard({
  title,
  meta,
  metaTitle,
  coverUrl,
  tint,
  glyph,
  active = false,
  playing = false,
  showEq = false,
  shape = 'square',
  showFab = true,
  playLabel,
  onToggle,
  disabled = false,
  foot,
  testId,
  dataTrackId,
  pressed,
  asButton = false,
  size = 'sm',
}: MediaCardProps) {
  const cls = `${styles.card}${size === 'sm' ? ` ${styles.cardSm}` : ''}${active ? ` ${styles.cardOn}` : ''}${disabled ? '' : ` ${styles.cardClickable}`}`
  const roundCls = shape === 'round' ? ` ${styles.coverRound}` : ''

  const coverInner = (
    <>
      {coverUrl ? (
        <img className={styles.img} src={coverUrl} alt="" />
      ) : (
        <span className={styles.glyph} aria-hidden="true">
          {glyph}
        </span>
      )}
      {playing && showEq && (
        <span className={styles.eq} aria-hidden="true">
          <i />
          <i />
          <i />
        </span>
      )}
      {!disabled && showFab && (
        <span className={styles.fab} aria-hidden="true">
          {playing ? <PauseIcon size={16} /> : <PlayIcon size={16} />}
        </span>
      )}
    </>
  )

  const body = (
    <>
      <div className={styles.title} title={title}>
        {title}
      </div>
      {meta !== undefined && (
        <div className={styles.meta} title={metaTitle}>
          {meta}
        </div>
      )}
      {foot}
    </>
  )

  if (asButton) {
    return (
      <button
        type="button"
        className={cls}
        onClick={onToggle}
        disabled={disabled}
        aria-pressed={pressed}
        aria-label={playLabel}
        data-testid={testId}
        data-track-id={dataTrackId}
      >
        <span className={`${styles.cover}${roundCls}`} style={coverUrl ? undefined : { background: tint }}>
          {coverInner}
        </span>
        {body}
      </button>
    )
  }

  return (
    <div className={cls} onClick={disabled ? undefined : onToggle} data-testid={testId} data-track-id={dataTrackId}>
      <button
        type="button"
        className={`${styles.coverBtn}${roundCls}`}
        style={coverUrl ? undefined : { background: tint }}
        onClick={(e) => {
          e.stopPropagation()
          onToggle()
        }}
        disabled={disabled}
        aria-label={playLabel}
      >
        {coverInner}
      </button>
      {body}
    </div>
  )
}
