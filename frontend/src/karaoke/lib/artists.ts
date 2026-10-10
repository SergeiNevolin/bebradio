import type { SongMeta } from './types'

export interface ParsedSong {
  /** Исполнители (пусто — не распознаны). Песня с коллаборацией входит в каждого. */
  artists: string[]
  /** Чищенное название для показа (без префикса исполнителя, ID-хвостов, доменных скобок). */
  title: string
}

/** Разделители соисполнителей — только с пробелами, чтобы не рвать слова (Xcho). */
const FEAT_RE = /\s*&\s*|\s*,\s*|\s*×\s*|\s+x\s+|\s+feat\.?\s+|\s+ft\.?\s+|\s+vs\.?\s+/i

function validName(s: string): boolean {
  const t = s.trim()
  return t.length >= 2 && /[\p{L}]/u.test(t)
}

function splitArtists(raw: string): string[] {
  const cleaned = raw.replace(/^by\s+/i, '').trim()
  if (!cleaned) return []
  return cleaned
    .split(FEAT_RE)
    .map((s) => s.trim())
    .filter(validName)
}

/** Исполнители песни: поле artist из метаданных, иначе разбор названия. */
export function songArtists(song: Pick<SongMeta, 'title'> & { artist?: string | null }): string[] {
  if (song.artist && song.artist.trim()) return splitArtists(song.artist)
  return parseSong(song.title).artists
}

/**
 * Разбирает «ИСПОЛНИТЕЛЬ - НАЗВАНИЕ» с файловым мусором:
 * ANNA_ASTI_-_Carica_(Pesni.CC), CUEA_-_YA_TEBYA_MOGNU_81980502,
 * «Индия & Xcho & MOT - Шадэ». Без разделителя — исполнителей нет.
 */
export function parseSong(raw: string): ParsedSong {
  const norm = raw.replace(/_/g, ' ').replace(/\s+/g, ' ').trim()
  const sep = norm.indexOf(' - ')
  let title = (sep >= 0 ? norm.slice(sep + 3) : norm).trim()
  // Доменные скобки в хвосте: Carica (Pesni.CC) → Carica.
  title = title.replace(/\s+\([^\s()]*\.[^\s()]*\)\s*$/, '').trim()
  // Длинные цифровые хвосты — ID файлов, не часть названия.
  title = title.replace(/\s+\d{5,}$/, '').trim()
  if (!title) title = norm
  const artists = sep >= 0 ? splitArtists(norm.slice(0, sep)) : []
  return { artists, title }
}

export interface ArtistEntry {
  /** Отображаемое имя (первый встречный вариант написания). */
  name: string
  songs: SongMeta[]
}

/** Группировка песен по исполнителям: больше песен — выше, далее по имени. */
export function groupArtists(songs: SongMeta[]): ArtistEntry[] {
  const map = new Map<string, ArtistEntry>()
  for (const song of songs) {
    for (const artist of songArtists(song)) {
      const key = artist.toLowerCase()
      const entry = map.get(key)
      if (entry) entry.songs.push(song)
      else map.set(key, { name: artist, songs: [song] })
    }
  }
  return [...map.values()].sort(
    (a, b) => b.songs.length - a.songs.length || a.name.localeCompare(b.name, 'ru'),
  )
}

function score(id: string, plays: Record<string, number>, fav: Set<string>): number {
  return (plays[id] ?? 0) * 2 + (fav.has(id) ? 1 : 0)
}

/**
 * Топ песен по исполнению: исполнения ×2 + избранное. При равных очках —
 * порядок каталога (сортировка стабильна). Всегда возвращает что-то
 * осмысленное даже без статистики.
 */
export function rankSongs(
  songs: SongMeta[],
  plays: Record<string, number>,
  favorites: string[],
  limit: number,
): SongMeta[] {
  const fav = new Set(favorites)
  return [...songs].sort((a, b) => score(b.id, plays, fav) - score(a.id, plays, fav)).slice(0, limit)
}

export function totalPlays(songs: SongMeta[], plays: Record<string, number>): number {
  return songs.reduce((sum, s) => sum + (plays[s.id] ?? 0), 0)
}
