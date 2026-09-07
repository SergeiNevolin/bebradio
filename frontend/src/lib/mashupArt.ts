// Shared visuals for mashups that have no cover art. A stable colour + a single
// glyph stand in for the missing image, the same way the design canvas mocks it.

const TINTS = [
  '#1f6feb',
  '#238636',
  '#8957e5',
  '#bc4c00',
  '#9e6a03',
  '#1a7f37',
  '#a40e26',
  '#0969da',
  '#bf3989',
  '#3d7a5d',
]

/** A deterministic tint for a mashup id, so its placeholder art never flickers. */
export function tintForId(id: string): string {
  let hash = 0
  for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) >>> 0
  return TINTS[hash % TINTS.length]
}

/** The single uppercase glyph shown on placeholder art. */
export function monoGlyph(title: string): string {
  const first = (title || '').trim()[0]
  return first ? first.toUpperCase() : '♪'
}
