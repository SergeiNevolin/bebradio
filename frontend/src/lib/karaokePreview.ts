import type { Track } from '../types'

/** Строка манифеста караоке (/api/karaoke/api/songs) с полями для превью. */
export interface KaraokePreview {
  id: string
  title: string
  language?: string | null
  duration?: number
  /** Относительный путь в статике караоке, напр. songs/{id}/minus.mp3. */
  audio?: string | null
  /** Оригинал с вокалом для превью; может отсутствовать. */
  original?: string | null
}

/** Превью — оригинал с вокалом, иначе минус; пути резолвятся через прокси /api/karaoke/. */
export function karaokePreviewUrl(song: KaraokePreview): string {
  const rel = song.original || song.audio || `songs/${song.id}/minus.mp3`
  return `/api/karaoke/${rel}`
}

/** Превью как элемент общей очереди: играет в глобальном плеере. */
export function karaokePreviewTrack(song: KaraokePreview): Track {
  return {
    id: `karaoke:${song.id}`,
    source: 'karaoke',
    title: song.title,
    artist: 'Караоке · превью',
    url: karaokePreviewUrl(song),
    thumbnail: '',
    duration: song.duration ?? 0,
    added_by: '',
    owner_id: '',
    size_bytes: 0,
    status: 'ready',
    has_cover: false,
    plays: 0,
    likes: 0,
    created_at: '',
  }
}
