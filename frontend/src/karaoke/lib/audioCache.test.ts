// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dropSongAudio, takeSongAudio, warmSongAudio } from './audioCache'

function okResponse(bytes: Uint8Array): Response {
  const view = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer
  return {
    ok: true,
    blob: () => Promise.resolve(new Blob([view])),
  } as Response
}

beforeEach(() => {
  dropSongAudio()
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => okResponse(new Uint8Array([1, 2, 3]))),
  )
  let n = 0
  vi.stubGlobal('URL', {
    createObjectURL: vi.fn(() => `blob:mock-${++n}`),
    revokeObjectURL: vi.fn(),
  })
})

afterEach(() => {
  dropSongAudio()
  vi.unstubAllGlobals()
})

describe('audioCache', () => {
  it('греет один раз на параллельные вызовы и отдаёт blob-URL', async () => {
    const [a, b] = await Promise.all([
      warmSongAudio('s1', 'minus', '/a.mp3'),
      warmSongAudio('s1', 'minus', '/a.mp3'),
    ])
    expect(a).toBe('blob:mock-1')
    expect(b).toBe('blob:mock-1')
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1)
    expect(takeSongAudio('s1', 'minus')).toBe('blob:mock-1')
  })

  it('take до готовности — null', () => {
    let resolveFetch!: (v: Response) => void
    vi.mocked(fetch).mockImplementationOnce(
      () => new Promise<Response>((res) => (resolveFetch = res)),
    )
    warmSongAudio('s1', 'minus', '/a.mp3')
    expect(takeSongAudio('s1', 'minus')).toBeNull()
    resolveFetch(okResponse(new Uint8Array([9])))
    return expect(warmSongAudio('s1', 'minus', '/a.mp3')).resolves.toBe('blob:mock-1')
  })

  it('новая песня сносит старую: аборт + revoke', async () => {
    await warmSongAudio('s1', 'minus', '/a.mp3')
    expect(takeSongAudio('s1', 'minus')).toBe('blob:mock-1')
    await warmSongAudio('s2', 'minus', '/b.mp3')
    expect(takeSongAudio('s1', 'minus')).toBeNull()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-1')
    expect(takeSongAudio('s2', 'minus')).toBe('blob:mock-2')
  })

  it('минус и оригинал живут рядом', async () => {
    await warmSongAudio('s1', 'minus', '/a.mp3')
    await warmSongAudio('s1', 'orig', '/o.mp3')
    expect(takeSongAudio('s1', 'minus')).toBe('blob:mock-1')
    expect(takeSongAudio('s1', 'orig')).toBe('blob:mock-2')
  })

  it('битый ответ — null, плеер остаётся на стриме', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({ ok: false } as Response)
    await expect(warmSongAudio('s1', 'minus', '/a.mp3')).resolves.toBeNull()
    expect(takeSongAudio('s1', 'minus')).toBeNull()
  })

  it('обрыв сети — null без реджекта', async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new TypeError('down'))
    await expect(warmSongAudio('s1', 'minus', '/a.mp3')).resolves.toBeNull()
  })

  it('пустое тело — null', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(okResponse(new Uint8Array([])))
    await expect(warmSongAudio('s1', 'minus', '/a.mp3')).resolves.toBeNull()
  })

  it('dropSongAudio(id) чистит только свою песню', async () => {
    await warmSongAudio('s1', 'minus', '/a.mp3')
    await warmSongAudio('s2', 'minus', '/b.mp3')
    dropSongAudio('s1')
    expect(takeSongAudio('s1', 'minus')).toBeNull()
    expect(takeSongAudio('s2', 'minus')).toBe('blob:mock-2')
  })
})
