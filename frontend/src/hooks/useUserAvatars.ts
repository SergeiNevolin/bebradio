import { useEffect, useState } from 'react'
import { api } from '../lib/api'

// Chat messages and the listeners list only carry a user id + name, so avatar
// URLs are resolved here on the client and cached module-wide. Each id is
// fetched at most once per session; anonymous listeners ("anon:<addr>") and
// empty ids are skipped because they have no profile.
const cache = new Map<string, string>()
const inflight = new Map<string, Promise<void>>()

const isReal = (id: string) => !!id && !id.startsWith('anon:')

function load(id: string): Promise<void> {
  if (cache.has(id)) return Promise.resolve()
  const existing = inflight.get(id)
  if (existing) return existing
  const p = api
    .getUser(id)
    .then((data) => {
      cache.set(id, data.user?.avatar_url || '')
    })
    .catch(() => {
      cache.set(id, '')
    })
    .finally(() => {
      inflight.delete(id)
    })
  inflight.set(id, p)
  return p
}

function snapshot(ids: string[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const id of ids) {
    const url = cache.get(id)
    if (url) out[id] = url
  }
  return out
}

/**
 * Resolves avatar URLs for the given user ids, returning a map of
 * `id -> avatarUrl` for the ids that have one. Re-fetches only ids not seen
 * before, so it is safe to pass a freshly mapped array on every render.
 */
export function useUserAvatars(ids: string[]): Record<string, string> {
  const key = Array.from(new Set(ids.filter(isReal))).sort().join(',')
  const [avatars, setAvatars] = useState<Record<string, string>>(() =>
    snapshot(key ? key.split(',') : []),
  )

  useEffect(() => {
    let cancelled = false
    const wanted = key ? key.split(',') : []
    const pending = wanted.filter((id) => !cache.has(id))

    if (pending.length === 0) {
      setAvatars((prev) => {
        const next = snapshot(wanted)
        return shallowEqual(prev, next) ? prev : next
      })
      return
    }

    Promise.all(pending.map(load)).then(() => {
      if (!cancelled) setAvatars(snapshot(wanted))
    })

    return () => {
      cancelled = true
    }
  }, [key])

  return avatars
}

function shallowEqual(a: Record<string, string>, b: Record<string, string>): boolean {
  const ak = Object.keys(a)
  if (ak.length !== Object.keys(b).length) return false
  return ak.every((k) => a[k] === b[k])
}
