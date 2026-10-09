/**
 * Тема и акцент у караоке общие с bebradio: их задаёт App.tsx через
 * переменные --bg/--surface/--primary на :root, а токены караоке
 * (karaoke/index.css) ссылаются на те же переменные. Здесь остались только
 * хелперы для canvas-отрисовки (волна, таймлайн, питч) — им нужны текущие
 * значения токенов.
 */

/** Значение CSS-переменной на <html> (для canvas-отрисовки). */
export function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

/** hex → rgba(...) с альфой (токены у нас hex-овые). */
export function hexRgba(hex: string, alpha: number): string {
  const m = /^#?([\da-f]{6})$/i.exec(hex)
  if (!m) return hex
  const n = parseInt(m[1], 16)
  return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${alpha})`
}