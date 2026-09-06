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

function darkenColor(hex: string, amount: number): string {
  const num = parseInt(hex.replace('#', ''), 16)
  const r = Math.max(0, (num >> 16) - amount)
  const g = Math.max(0, ((num >> 8) & 0x00FF) - amount)
  const b = Math.max(0, (num & 0x0000FF) - amount)
  return `#${(r << 16 | g << 8 | b).toString(16).padStart(6, '0')}`
}

function hexToRgb(hex: string): { r: number; g: number; b: number } | null {
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex)
  return result
    ? { r: parseInt(result[1], 16), g: parseInt(result[2], 16), b: parseInt(result[3], 16) }
    : null
}

export type Theme = 'light' | 'dark'

const THEME_KEY = 'theme'

export function getStoredTheme(): Theme {
  const stored = localStorage.getItem(THEME_KEY)
  if (stored === 'dark' || stored === 'light') return stored
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function applyTheme(theme: Theme) {
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem(THEME_KEY, theme)
}

export function toggleTheme(): Theme {
  const current = getStoredTheme()
  const next = current === 'dark' ? 'light' : 'dark'
  applyTheme(next)
  return next
}

export function applyAccent(color: string): void {
  const root = document.documentElement
  if (color) {
    root.style.setProperty('--primary', color)
    root.style.setProperty('--primary-hover', darkenColor(color, 20))
    const rgb = hexToRgb(color)
    if (rgb) {
      root.style.setProperty('--primary-r', String(rgb.r))
      root.style.setProperty('--primary-g', String(rgb.g))
      root.style.setProperty('--primary-b', String(rgb.b))
    }
    try {
      localStorage.setItem(STORAGE_KEY, color)
    } catch {
      /* ignore */
    }
  } else {
    root.style.removeProperty('--primary')
    root.style.removeProperty('--primary-hover')
    root.style.removeProperty('--primary-r')
    root.style.removeProperty('--primary-g')
    root.style.removeProperty('--primary-b')
    try {
      localStorage.removeItem(STORAGE_KEY)
    } catch {
      /* ignore */
    }
  }
}
