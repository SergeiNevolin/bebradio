/**
 * Предзагрузка аудио песни в Blob до старта пения.
 *
 * Боль standalone-файла: seek в непрослушанное место дёргает Range по цепочке
 * nginx → karaoke-service → silo, и на мобильной сети это слышимая дырка.
 * Греем минус (и оригинал) заранее — в фоне, пока юзер выбирает микрофон и
 * идёт отсчёт, — а плеер подменяет src на blob-URL: дальше все перемотки
 * мгновенные, без сети вообще.
 *
 * Контракт:
 * - warmSongAudio(songId, role, url) — fire-and-forget, дедуплицирует
 *   параллельные грелки; warming под другой songId сносит старые записи
 *   (аборт докачки + revoke URL — в памяти не больше одной песни).
 * - takeSongAudio(songId, role) — готовый blob-URL или null (ещё качается).
 * - dropSongAudio(songId?) — отменить и забыть (без id — всё).
 * Промисы не реджектятся никогда: неуспех = null, плеер остаётся на стриме.
 */

interface WarmEntry {
  key: string
  songId: string
  url: string | null
  controller: AbortController
  promise: Promise<string | null>
}

const entries = new Map<string, WarmEntry>()

function revoke(entry: WarmEntry): void {
  try {
    entry.controller.abort()
  } catch {
    /* ignore */
  }
  if (entry.url) {
    try {
      URL.revokeObjectURL(entry.url)
    } catch {
      /* ignore */
    }
    entry.url = null
  }
}

function dropKeys(predicate: (key: string) => boolean): void {
  for (const [key, entry] of entries) {
    if (predicate(key)) {
      revoke(entry)
      entries.delete(key)
    }
  }
}

export function warmSongAudio(songId: string, role: 'minus' | 'orig', url: string): Promise<string | null> {
  const key = `${songId}:${role}`
  const existing = entries.get(key)
  if (existing) return existing.promise
  // Одна песня за раз: остатки прошлой отменяем и выкидываем из памяти.
  dropKeys((k) => !k.startsWith(`${songId}:`))

  const controller = new AbortController()
  const entry: WarmEntry = { key, songId, url: null, controller, promise: Promise.resolve(null) }
  entries.set(key, entry)
  entry.promise = (async (): Promise<string | null> => {
    try {
      const r = await fetch(url, { signal: controller.signal })
      if (!r.ok) return null
      const blob = await r.blob()
      if (controller.signal.aborted || blob.size === 0) return null
      const objectUrl = URL.createObjectURL(blob)
      // Пока качали — песню могли сменить и запись снести.
      if (entries.get(key) !== entry || controller.signal.aborted) {
        try {
          URL.revokeObjectURL(objectUrl)
        } catch {
          /* ignore */
        }
        return null
      }
      entry.url = objectUrl
      return objectUrl
    } catch {
      return null
    }
  })()
  return entry.promise
}

/** Готовый blob-URL (подменять src) или null — ещё качается / не грели / отвалилось. */
export function takeSongAudio(songId: string, role: 'minus' | 'orig'): string | null {
  return entries.get(`${songId}:${role}`)?.url ?? null
}

/** Отменить и забыть: конкретную песню либо всё (без аргумента). */
export function dropSongAudio(songId?: string): void {
  if (songId === undefined) dropKeys(() => true)
  else dropKeys((k) => k.startsWith(`${songId}:`))
}
