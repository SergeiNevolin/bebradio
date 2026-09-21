import { memo } from 'react'
import { monoGlyph, tintForId } from '../lib/mashupArt'
import styles from './TrackArt.module.css'

interface TrackArtProps {
  id: string
  title: string
  thumbnail?: string
  /** Square size in px (overridden by height when set). */
  size: number
  height?: number
  radius?: number
  /**
   * Box class from the consumer (Queue/Player/search rows): carries the
   * layout slot. Applied to both the img and the placeholder so the shape
   * stays identical with or without artwork.
   */
  className?: string
}

/**
 * Track artwork with a built-in placeholder: when a track (usually a mashup
 * without a cover) has no thumbnail, render a deterministic tint + title
 * glyph instead of an empty hole, so every source looks the same shape.
 */
function TrackArt({ id, title, thumbnail, size, height, radius = 6, className }: TrackArtProps) {
  const h = height ?? size
  if (thumbnail) {
    return (
      <img
        src={thumbnail}
        alt=""
        width={size}
        height={h}
        style={{ borderRadius: radius }}
        className={className}
      />
    )
  }
  return (
    <span
      aria-hidden="true"
      className={`${styles.fallback}${className ? ` ${className}` : ''}`}
      style={{
        background: tintForId(id),
        width: size,
        height: h,
        borderRadius: radius,
        fontSize: Math.round(Math.min(size, h) * 0.45),
      }}
    >
      {monoGlyph(title)}
    </span>
  )
}

export default memo(TrackArt)
