import { useEffect, useRef, useState } from 'react'
import { ACCENT_PRESETS, DEFAULT_ACCENT, applyAccent, getStoredAccent } from '../lib/theme'

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
    <div className="accent-picker" ref={ref}>
      <button
        type="button"
        className="accent-swatch"
        onClick={() => setOpen((o) => !o)}
        title="Accent color"
        aria-label="Accent color"
      />
      {open && (
        <div className="accent-pop" role="menu">
          <div className="accent-grid">
            {ACCENT_PRESETS.map((preset) => (
              <button
                key={preset.name}
                type="button"
                className={`accent-dot${accent === preset.value ? ' is-active' : ''}`}
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
          <label className="accent-custom">
            <span>Свой цвет</span>
            <input
              type="color"
              value={accent || DEFAULT_ACCENT}
              onChange={(e) => setAccent(e.target.value)}
            />
          </label>
        </div>
      )}
    </div>
  )
}
