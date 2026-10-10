// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useKaraoke } from './store'

const song = (id: string) => ({
  id,
  title: id,
  audio: `songs/${id}/minus.mp3`,
  lines: 0,
  duration: 100,
  segments: [],
  pitch: null,
  waveform: null,
})

beforeEach(() => {
  localStorage.clear()
  useKaraoke.setState({
    screen: 'catalog',
    songs: [],
    song: null,
    loadingSong: false,
    favorites: [],
    recent: [],
  })
})

describe('useKaraoke', () => {
  it('навигация каталог <-> песня', () => {
    expect(useKaraoke.getState().screen).toBe('catalog')
    useKaraoke.getState().openSong(song('a'))
    expect(useKaraoke.getState().screen).toBe('player')
    expect(useKaraoke.getState().song?.id).toBe('a')
    useKaraoke.getState().back()
    expect(useKaraoke.getState().screen).toBe('catalog')
    expect(useKaraoke.getState().song).toBe(null)
  })

  it('setSongs / setLoadingSong', () => {
    useKaraoke.getState().setSongs([{ id: 'a', title: 'a', audio: 'x', lines: 1, duration: 10 }])
    expect(useKaraoke.getState().songs).toHaveLength(1)
    useKaraoke.getState().setLoadingSong(true)
    expect(useKaraoke.getState().loadingSong).toBe(true)
  })

  it('избранное: toggle + персист', () => {
    const s = useKaraoke.getState()
    s.toggleFavorite('a')
    expect(useKaraoke.getState().favorites).toEqual(['a'])
    expect(JSON.parse(localStorage.getItem('karaoke:favorites')!)).toEqual(['a'])
    useKaraoke.getState().toggleFavorite('a')
    expect(useKaraoke.getState().favorites).toEqual([])
  })

  it('recent: дедуп, свежие сверху, кап 20', () => {
    const s = useKaraoke.getState()
    s.pushRecent('a')
    s.pushRecent('b')
    s.pushRecent('a')
    expect(useKaraoke.getState().recent.slice(0, 2)).toEqual(['a', 'b'])
    for (let i = 0; i < 25; i++) useKaraoke.getState().pushRecent(`s${i}`)
    expect(useKaraoke.getState().recent).toHaveLength(20)
    expect(useKaraoke.getState().recent[0]).toBe('s24')
  })

  it('битый localStorage — дефолты, не падаем', async () => {
    localStorage.setItem('karaoke:favorites', '???')
    localStorage.setItem('karaoke:recent', '[1,2]')
    vi.resetModules()
    const m = await import('./store')
    expect(m.useKaraoke.getState().favorites).toEqual([])
    expect(m.useKaraoke.getState().recent).toEqual([])
  })

  it('removeSong чистит каталог, избранное, недавние и счётчики', () => {
    const s = useKaraoke.getState()
    s.setSongs([{ id: 'a', title: 'a', audio: 'x', lines: 1, duration: 10 }])
    s.toggleFavorite('a')
    s.pushRecent('a')
    s.openSong(song('a'))
    s.removeSong('a')
    const st = useKaraoke.getState()
    expect(st.songs).toEqual([])
    expect(st.favorites).toEqual([])
    expect(st.recent).toEqual([])
    expect(st.plays['a']).toBeUndefined()
  })

  it('updateSongMeta правит каталог и открытую песню', () => {
    const s = useKaraoke.getState()
    s.setSongs([
      { id: 'a', title: 'a', audio: 'x', lines: 1, duration: 10 },
      { id: 'b', title: 'b', audio: 'x', lines: 1, duration: 10 },
    ])
    s.openSong(song('a'))
    s.updateSongMeta('a', 'Новое', 'Автор')
    const st = useKaraoke.getState()
    expect(st.songs.find((x) => x.id === 'a')).toMatchObject({ title: 'Новое', artist: 'Автор' })
    expect(st.songs.find((x) => x.id === 'b')).toMatchObject({ title: 'b' })
    expect(st.song).toMatchObject({ title: 'Новое', artist: 'Автор' })
  })

  it('editRequest/editorOpen: заявка и флаг редактора', () => {
    const s = useKaraoke.getState()
    expect(s.editRequest).toBeNull()
    expect(s.editorOpen).toBe(false)
    s.requestEdit('a')
    expect(useKaraoke.getState().editRequest).toBe('a')
    s.consumeEditRequest()
    expect(useKaraoke.getState().editRequest).toBeNull()
    s.setEditorOpen(true)
    expect(useKaraoke.getState().editorOpen).toBe(true)
  })
})
