// Accent colour ("primary") customisation.
//
// The stylesheet defines --primary / --primary-hover per theme. When the user
// picks a custom accent we set those two custom properties inline on <html>,
// which overrides the stylesheet; clearing the choice removes the inline
// properties and the stylesheet defaults take over again.

const STORAGE_KEY = 'accent'

// The stylesheet's light-theme --primary. Used only as the swatch colour for
// the "default" preset in the picker.
export const DEFAULT_ACCENT = '#16a34a'

export interface AccentPreset {
  name: string
  /** Empty string means "stylesheet default". */
  value: string
}

export const ACCENT_PRESETS: AccentPreset[] = [
  { name: 'Green', value: '' },
  { name: 'Blue', value: '#2563eb' },
  { name: 'Violet', value: '#7c3aed' },
  { name: 'Pink', value: '#db2777' },
  { name: 'Orange', value: '#ea580c' },
  { name: 'Red', value: '#dc2626' },
  { name: 'Teal', value: '#0d9488' },
  { name: 'Slate', value: '#475569' },
]

export function getStoredAccent(): string {
  try {
    return localStorage.getItem(STORAGE_KEY) || ''
  } catch {
    return ''
  }
}

export function applyAccent(color: string): void {
  const root = document.documentElement
  if (color) {
    root.style.setProperty('--primary', color)
    root.style.setProperty('--primary-hover', `color-mix(in srgb, ${color}, #000 14%)`)
    try {
      localStorage.setItem(STORAGE_KEY, color)
    } catch {
      /* ignore */
    }
  } else {
    root.style.removeProperty('--primary')
    root.style.removeProperty('--primary-hover')
    try {
      localStorage.removeItem(STORAGE_KEY)
    } catch {
      /* ignore */
    }
  }
}
