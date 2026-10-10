import { create } from 'zustand'
import type { SongData, SongMeta } from './lib/types'

type Screen = 'catalog' | 'player'

function readJSON(key: string, fallback: string[]): string[] {
  try {
    const raw = localStorage.getItem(key)
    const v = raw ? (JSON.parse(raw) as unknown) : fallback
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : fallback
  } catch {
    return fallback
  }
}
function readPlays(): Record<string, number> {
  try {
    const raw = localStorage.getItem(PLAYS_KEY)
    const v = raw ? (JSON.parse(raw) as unknown) : {}
    if (v && typeof v === 'object' && !Array.isArray(v)) {
      const out: Record<string, number> = {}
      for (const [k, n] of Object.entries(v as Record<string, unknown>)) {
        if (typeof n === 'number' && Number.isFinite(n) && n > 0) out[k] = Math.floor(n)
      }
      return out
    }
  } catch {
    /* ignore */
  }
  return {}
}

const FAV_KEY = 'karaoke:favorites'

const RECENT_KEY = 'karaoke:recent'

const PLAYS_KEY = 'karaoke:plays'

const MAX_RECENT = 20
function persist(favorites: string[], recent: string[]) {
  try {
    localStorage.setItem(FAV_KEY, JSON.stringify(favorites))
    localStorage.setItem(RECENT_KEY, JSON.stringify(recent))
  } catch {
    /* приватный режим — просто не запоминаем */
  }
}

function persistPlays(plays: Record<string, number>) {
  try {
    localStorage.setItem(PLAYS_KEY, JSON.stringify(plays))
  } catch {
    /* приватный режим — просто не запоминаем */
  }
}

interface KaraokeState {
  screen: Screen
  songs: SongMeta[]
  song: SongData | null
  loadingSong: boolean
  favorites: string[]
  recent: string[]
  /** Сколько раз пели каждую песню — основа подборки «Популярное». */
  plays: Record<string, number>
  setSongs: (s: SongMeta[]) => void
  openSong: (s: SongData) => void
  back: () => void
  setLoadingSong: (v: boolean) => void
  toggleFavorite: (id: string) => void
  pushRecent: (id: string) => void
  /** Удалить песню из всего локального состояния (каталог, избранное, недавние, счётчики). */
  removeSong: (id: string) => void
  /** Обновить название/автора песни в каталоге и в открытой песне. */
  updateSongMeta: (id: string, title: string, artist: string | null) => void
  /** Идёт ли сейчас пение в караоке — общий плеер при этом прячется и молчит. */
  soundActive: boolean
  setSoundActive: (v: boolean) => void
  /** Просьба открыть редактор для песни (кнопка из превью). */
  editRequest: string | null
  requestEdit: (id: string) => void
  consumeEditRequest: () => void
  /** Редактор текста открыт поверх плеера. */
  editorOpen: boolean
  setEditorOpen: (v: boolean) => void
}

export const useKaraoke = create<KaraokeState>((set, get) => ({
  screen: 'catalog',
  songs: [],
  song: null,
  loadingSong: false,
  favorites: readJSON(FAV_KEY, []),
  recent: readJSON(RECENT_KEY, []),
  plays: readPlays(),
  soundActive: false,
  editRequest: null,
  editorOpen: false,
  setSongs: (songs) => set({ songs }),
  openSong: (song) => {
    if (song.id) {
      const plays = { ...get().plays, [song.id]: (get().plays[song.id] ?? 0) + 1 }
      persistPlays(plays)
      set({ song, screen: 'player', plays })
    } else {
      set({ song, screen: 'player' })
    }
  },
  back: () => set({ screen: 'catalog', song: null }),
  setLoadingSong: (v) => set({ loadingSong: v }),
  toggleFavorite: (id) => {
    const has = get().favorites.includes(id)
    const favorites = has ? get().favorites.filter((f) => f !== id) : [...get().favorites, id]
    persist(favorites, get().recent)
    set({ favorites })
  },
  pushRecent: (id) => {
    const recent = [id, ...get().recent.filter((r) => r !== id)].slice(0, MAX_RECENT)
    persist(get().favorites, recent)
    set({ recent })
  },
  removeSong: (id) => {
    const favorites = get().favorites.filter((f) => f !== id)
    const recent = get().recent.filter((r) => r !== id)
    persist(favorites, recent)
    const { [id]: _drop, ...plays } = get().plays
    persistPlays(plays)
    set({ songs: get().songs.filter((s) => s.id !== id), favorites, recent, plays })
  },
  updateSongMeta: (id, title, artist) => {
    const songs = get().songs.map((s) => (s.id === id ? { ...s, title, artist } : s))
    const cur = get().song
    set({ songs, song: cur && cur.id === id ? { ...cur, title, artist } : cur })
  },
  setSoundActive: (v) => set({ soundActive: v }),
  requestEdit: (id) => set({ editRequest: id }),
  consumeEditRequest: () => set({ editRequest: null }),
  setEditorOpen: (v) => set({ editorOpen: v }),
}))
