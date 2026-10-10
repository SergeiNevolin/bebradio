import styles from './Avatar.module.css'

interface AvatarProps {
  name: string
  src?: string
  size?: number
  className?: string
}

// Small round user avatar. Falls back to the first letter of the name on a
// solid background when no image URL is available (or it fails to load).
export default function Avatar({ name, src, size = 28, className = '' }: AvatarProps) {
  const initial = (name?.trim()?.[0] ?? '?').toUpperCase()
  return (
    <span
      className={`${styles.avatar} ${className}`}
      style={{ width: size, height: size, fontSize: Math.round(size * 0.42) }}
      aria-hidden="true"
    >
      {src ? <img src={src} alt="" loading="lazy" /> : <span>{initial}</span>}
    </span>
  )
}
