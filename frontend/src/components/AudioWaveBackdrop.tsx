import { useEffect, useRef } from 'react'

interface AudioWaveBackdropProps {
  /** A track is loaded — without this the backdrop stays a plain colour. */
  active: boolean
  /** Playback is running: waves swell and flow; otherwise they calm down. */
  isPlaying: boolean
  /** Current track id, used to seed subtle per-track variation. */
  seed: string
}

/** Stable 0–1 value from a string (FNV-1a). */
function hashUnit(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i += 1) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return ((h >>> 0) % 100000) / 100000
}

function readColor(name: string, fallback: string): string {
  try {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    return v || fallback
  } catch {
    return fallback
  }
}

function withAlpha(color: string, a: number): string {
  const c = color.trim()
  if (c.startsWith('#')) {
    let hex = c.slice(1)
    if (hex.length === 3) hex = hex.split('').map((ch) => ch + ch).join('')
    hex = hex.padEnd(6, '0').slice(0, 6)
    const r = parseInt(hex.slice(0, 2), 16)
    const g = parseInt(hex.slice(2, 4), 16)
    const b = parseInt(hex.slice(4, 6), 16)
    return `rgba(${r}, ${g}, ${b}, ${a})`
  }
  if (c.startsWith('rgba(')) return c.replace(/[\d.]+\s*\)$/, `${a})`)
  if (c.startsWith('rgb(')) return c.replace('rgb(', 'rgba(').replace(')', `, ${a})`)
  return `rgba(34, 197, 94, ${a})`
}

function prefersReducedMotion(): boolean {
  try {
    return (
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    )
  } catch {
    return false
  }
}

const BANDS = 4
const IDLE_ENERGY = 0.15

/**
 * A calm, flowing wave field rendered full-bleed behind the room, blurred
 * and dimmed. It is not a true FFT visualiser — the stream is cross-origin
 * and Web Audio would mute it — but the motion tracks play/pause so it reads
 * as one.
 */
export default function AudioWaveBackdrop({ active, isPlaying, seed }: AudioWaveBackdropProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const rafRef = useRef(0)
  const energyRef = useRef(IDLE_ENERGY)
  const clockRef = useRef(0)
  const lastRef = useRef(0)
  const idleDrawRef = useRef(0)
  const playingRef = useRef(isPlaying)
  const seedRef = useRef(hashUnit(seed))

  playingRef.current = isPlaying
  useEffect(() => {
    seedRef.current = hashUnit(seed)
  }, [seed])

  useEffect(() => {
    if (!active) return undefined
    const canvas = canvasRef.current
    if (!canvas) return undefined

    let ctx: CanvasRenderingContext2D | null = null
    try {
      ctx = canvas.getContext('2d')
    } catch {
      ctx = null
    }
    if (!ctx) return undefined
    const context = ctx

    const reduced = prefersReducedMotion()
    let colorA = readColor('--primary', '#22c55e')
    let colorB = readColor('--primary-hover', '#16a34a')
    let colorAge = 0

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      canvas.width = Math.max(1, Math.floor(canvas.clientWidth * dpr))
      canvas.height = Math.max(1, Math.floor(canvas.clientHeight * dpr))
      context.setTransform(dpr, 0, 0, dpr, 0, 0)
    }
    resize()
    window.addEventListener('resize', resize)

    const draw = (now: number) => {
      rafRef.current = requestAnimationFrame(draw)

      const dt = lastRef.current ? Math.min((now - lastRef.current) / 1000, 0.05) : 0.016
      lastRef.current = now

      const target = playingRef.current ? 1 : IDLE_ENERGY
      energyRef.current += (target - energyRef.current) * Math.min(dt * 2.2, 1)
      const e = energyRef.current

      // Idle and settled: hold the last frame, redraw only occasionally.
      if (!playingRef.current && Math.abs(e - IDLE_ENERGY) < 0.01) {
        if (now - idleDrawRef.current < 200) return
        idleDrawRef.current = now
      }

      if (!reduced) clockRef.current += dt * (0.12 + e * 0.9)
      const t = clockRef.current

      colorAge += dt
      if (colorAge > 1) {
        colorAge = 0
        colorA = readColor('--primary', colorA)
        colorB = readColor('--primary-hover', colorB)
      }

      const w = canvas.clientWidth || canvas.width
      const h = canvas.clientHeight || canvas.height
      context.clearRect(0, 0, w, h)
      if (w < 2 || h < 2) return
      context.globalCompositeOperation = 'lighter'

      const sd = seedRef.current
      const step = Math.max(6, Math.floor(w / 140))

      for (let i = 0; i < BANDS; i += 1) {
        const f = i / (BANDS - 1)
        const centre = h * (0.3 + 0.44 * f) + Math.sin(t * 0.15 + i * 1.3) * h * 0.03
        const amp = (h * 0.05 + h * 0.13 * (1 - f)) * (0.22 + 0.78 * e)
        const thick = h * (0.05 + 0.035 * f)
        const speed = 0.45 + i * 0.22 + sd * 0.3
        const k1 = 1.3 + i * 0.5
        const k2 = 3.0 + i * 0.7
        const ph = t * speed + i * 1.7 + sd * 6.283

        const edge = (x: number, offset: number) => {
          const u = x / w
          return (
            centre +
            offset +
            Math.sin(u * k1 * Math.PI * 2 + ph) * amp +
            Math.sin(u * k2 * Math.PI * 2 - ph * 0.7) * amp * 0.4
          )
        }

        context.beginPath()
        context.moveTo(-step, edge(-step, 0))
        for (let x = 0; x <= w + step; x += step) context.lineTo(x, edge(x, 0))
        for (let x = w + step; x >= -step; x -= step) context.lineTo(x, edge(x, thick))
        context.closePath()

        const col = i % 2 ? colorB : colorA
        const grad = context.createLinearGradient(0, centre - amp, 0, centre + amp + thick)
        grad.addColorStop(0, withAlpha(col, 0.015))
        grad.addColorStop(0.5, withAlpha(col, 0.16 * (0.35 + 0.65 * e)))
        grad.addColorStop(1, withAlpha(col, 0.015))
        context.fillStyle = grad
        context.fill()
      }

      context.globalCompositeOperation = 'source-over'
    }

    rafRef.current = requestAnimationFrame(draw)

    return () => {
      cancelAnimationFrame(rafRef.current)
      window.removeEventListener('resize', resize)
      lastRef.current = 0
    }
  }, [active])

  if (!active) return null

  return (
    <div className="wave-backdrop" aria-hidden="true">
      <canvas ref={canvasRef} className="wave-canvas" />
    </div>
  )
}
