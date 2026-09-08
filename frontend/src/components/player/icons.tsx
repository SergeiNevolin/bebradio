import type { ReactNode } from 'react'

/**
 * One inline-SVG component per player glyph, all sharing the signature
 * `({ size = 18 }) => JSX.Element`. Paths are lifted verbatim from icon sets
 * already bundled in the app — Material Icons (transport, shuffle, repeat,
 * queue, volume; the volume trio matches what `VolumeControl` shipped before)
 * and GitHub Octicons (heart, chevrons). `fill="currentColor"` lets each icon
 * inherit the button's colour, so a custom accent from `lib/theme.ts` works
 * with no extra wiring.
 */

interface IconProps {
  size?: number
}

function Icon({
  size = 18,
  viewBox,
  children,
}: {
  size?: number
  viewBox: string
  children: ReactNode
}) {
  return (
    <svg width={size} height={size} viewBox={viewBox} fill="currentColor" aria-hidden="true">
      {children}
    </svg>
  )
}

/* ── Material transport ─────────────────────────────────────────────── */

export function PlayIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M8 5.14v13.72a1 1 0 0 0 1.5.86l11-6.86a1 1 0 0 0 0-1.72l-11-6.86A1 1 0 0 0 8 5.14z" />
    </Icon>
  )
}

export function PauseIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M7 4h4v16H7zM13 4h4v16h-4z" />
    </Icon>
  )
}

export function PrevIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M7 5a1 1 0 0 1 2 0v5.4l8.5-5.26A1 1 0 0 1 19 6v12a1 1 0 0 1-1.5.86L9 13.6V19a1 1 0 0 1-2 0z" />
    </Icon>
  )
}

export function NextIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M17 5a1 1 0 0 0-2 0v5.4L6.5 5.14A1 1 0 0 0 5 6v12a1 1 0 0 0 1.5.86L15 13.6V19a1 1 0 0 0 2 0z" />
    </Icon>
  )
}

/* ── Material toggles ──────────────────────────────────────────────── */

export function ShuffleIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M10.59 9.17 6.12 4.7 4.7 6.12l4.47 4.46 1.42-1.41zM14.5 4l2.09 2.09L4.7 17.88 6.12 19.3 17.91 7.5 20 9.59V4zM14.83 13.41l-1.41 1.41 3.13 3.13L14.5 20H20v-5.5l-2.04 2.04z" />
    </Icon>
  )
}

export function RepeatIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M7 7h10v3l4-4-4-4v3H5v6h2zm10 10H7v-3l-4 4 4 4v-3h12v-6h-2z" />
    </Icon>
  )
}

export function RepeatOneIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M7 7h10v3l4-4-4-4v3H5v6h2zm10 10H7v-3l-4 4 4 4v-3h12v-6h-2zM13 15V9h-1l-2 1v1h1.5v4z" />
    </Icon>
  )
}

export function QueueIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M15 6H3v2h12zM15 10H3v2h12zM3 16h8v-2H3zM17 6v8.18A3 3 0 1 0 19 17V8h3V6z" />
    </Icon>
  )
}

/* ── Material volume (identical paths to the former inline SVGs) ────── */

export function VolumeHighIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z" />
    </Icon>
  )
}

export function VolumeLowIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M18.5 12c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM5 9v6h4l5 5V4L9 9H5z" />
    </Icon>
  )
}

export function VolumeMutedIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 24 24">
      <path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51A8.796 8.796 0 0021 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06a8.99 8.99 0 003.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z" />
    </Icon>
  )
}

/* ── Octicons ──────────────────────────────────────────────────────── */

export function HeartIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 16 16">
      <path d="M4.25 2.5c-1.336 0-2.75 1.164-2.75 3 0 2.15 1.58 4.144 3.365 5.682A20.565 20.565 0 0 0 8 13.393a20.561 20.561 0 0 0 3.135-2.211C12.92 9.644 14.5 7.65 14.5 5.5c0-1.836-1.414-3-2.75-3-1.373 0-2.609.986-3.029 2.416a.75.75 0 0 1-1.442 0C6.859 3.486 5.623 2.5 4.25 2.5Z" />
    </Icon>
  )
}

export function HeartFillIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 16 16">
      <path d="m8 14.25.345.666a.75.75 0 0 1-.69 0l-.008-.004-.018-.01a7.152 7.152 0 0 1-.31-.17 22.055 22.055 0 0 1-3.434-2.414C2.045 10.731 0 8.35 0 5.5 0 2.836 2.086 1 4.25 1 5.797 1 7.153 1.802 8 3.02 8.847 1.802 10.203 1 11.75 1 13.914 1 16 2.836 16 5.5c0 2.85-2.045 5.231-3.885 6.818a22.066 22.066 0 0 1-3.744 2.584l-.018.01-.006.003h-.002Z" />
    </Icon>
  )
}

export function ChevronUpIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 16 16">
      <path d="M3.22 10.53a.749.749 0 0 1 0-1.06l4.25-4.25a.749.749 0 0 1 1.06 0l4.25 4.25a.749.749 0 1 1-1.06 1.06L8 6.811 4.28 10.53a.749.749 0 0 1-1.06 0Z" />
    </Icon>
  )
}

export function ChevronDownIcon({ size }: IconProps) {
  return (
    <Icon size={size} viewBox="0 0 16 16">
      <path d="M12.78 5.47a.749.749 0 0 1 0 1.06l-4.25 4.25a.749.749 0 0 1-1.06 0L3.22 6.53a.749.749 0 1 1 1.06-1.06L8 9.189l3.72-3.719a.749.749 0 0 1 1.06 0Z" />
    </Icon>
  )
}
