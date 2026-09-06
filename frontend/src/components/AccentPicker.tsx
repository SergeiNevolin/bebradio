import { useEffect, useRef, useState } from 'react'
import { ACCENT_PRESETS, DEFAULT_ACCENT, applyAccent, getStoredAccent } from '../lib/theme'
import styles from './AccentPicker.module.css'

export default function AccentPicker() {
  const [open, setOpen] = useState(false)
  const [accent, setAccent] = useState(getStoredAccent)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    applyAccent(accent)
  }, [accent])

  useEffect(() => {
    if (!open) return
    const onDocClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div className={styles.accentPicker} ref={ref}>
      <button
        type="button"
        className={styles.accentSwatch}
        onClick={() => setOpen((o) => !o)}
        title="Accent color"
        aria-label="Accent color"
      />
      {open && (
        <div className={styles.accentPop} role="menu">
          <div className={styles.accentGrid}>
            {ACCENT_PRESETS.map((preset) => (
              <button
                key={preset.name}
                type="button"
                className={`${styles.accentDot}${accent === preset.value ? ` ${styles.accentDotActive}` : ''}`}
                style={{ background: preset.value || DEFAULT_ACCENT }}
                title={preset.name}
                aria-label={preset.name}
                onClick={() => {
                  setAccent(preset.value)
                  setOpen(false)
                }}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
