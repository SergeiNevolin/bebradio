// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { groupArtists, parseSong, rankSongs, totalPlays } from './artists'
import type { SongMeta } from './types'

function meta(over: Partial<SongMeta> & { id: string; title: string }): SongMeta {
  return { audio: '', lines: 1, duration: 60, ...over }
}

describe('parseSong', () => {
  const cases: Array<[string, string[], string]> = [
    ['ANNA_ASTI_-_Carica_(Pesni.CC)', ['ANNA ASTI'], 'Carica'],
    ['Aarne_Toxi_Big_Baby_Tape_-_NOBODY_80172673', ['Aarne Toxi Big Baby Tape'], 'NOBODY'],
    ['Artur_Pirozhkov_-_CHika_55936287', ['Artur Pirozhkov'], 'CHika'],
    ['CUEA_-_YA_TEBYA_MOGNU_81980502', ['CUEA'], 'YA TEBYA MOGNU'],
    ['By Индия & Xcho & MOT - Шадэ', ['Индия', 'Xcho', 'MOT'], 'Шадэ'],
    ['DJ A feat. DJ B - Track', ['DJ A', 'DJ B'], 'Track'],
    ['Ленинград - WWW', ['Ленинград'], 'WWW'],
    ['ABBA - 1999', ['ABBA'], '1999'],
    ['audio', [], 'audio'],
    ['Sunset Boulevard', [], 'Sunset Boulevard'],
    ['12345 - Song', [], 'Song'],
  ]
  for (const [raw, artists, title] of cases) {
    it(`«${raw}»`, () => {
      expect(parseSong(raw)).toEqual({ artists, title })
    })
  }
})

describe('groupArtists', () => {
  it('группирует и сортирует по числу песен, коллаборации — в каждого', () => {
    const songs = [
      meta({ id: 'a1', title: 'ANNA ASTI - Uno' }),
      meta({ id: 'a2', title: 'ANNA ASTI - Dos' }),
      meta({ id: 'c1', title: 'CUEA & ANNA ASTI - Duo' }),
      meta({ id: 'x1', title: 'audio' }),
    ]
    const groups = groupArtists(songs)
    expect(groups.map((g) => g.name)).toEqual(['ANNA ASTI', 'CUEA'])
    expect(groups[0].songs.map((s) => s.id)).toEqual(['a1', 'a2', 'c1'])
    expect(groups[1].songs.map((s) => s.id)).toEqual(['c1'])
  })
})

describe('rankSongs', () => {
  const songs = [meta({ id: 'a', title: 'A' }), meta({ id: 'b', title: 'B' }), meta({ id: 'c', title: 'C' })]

  it('исполнения важнее избранного, лимит режет', () => {
    const ranked = rankSongs(songs, { b: 3, c: 1 }, ['a'], 2)
    expect(ranked.map((s) => s.id)).toEqual(['b', 'c'])
  })

  it('без статистики — порядок каталога', () => {
    expect(rankSongs(songs, {}, [], 8).map((s) => s.id)).toEqual(['a', 'b', 'c'])
  })

  it('totalPlays суммирует', () => {
    expect(totalPlays(songs, { a: 2, c: 1 })).toBe(3)
  })
})
