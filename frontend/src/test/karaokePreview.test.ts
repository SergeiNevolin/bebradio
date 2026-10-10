import { describe, expect, it } from 'vitest'
import { karaokePreviewTrack, karaokePreviewUrl } from '../lib/karaokePreview'

describe('karaokePreviewUrl', () => {
  it('prefers original, falls back to minus', () => {
    expect(karaokePreviewUrl({ id: 't', title: 'T', original: 'songs/t/original.mp3' })).toBe(
      '/api/karaoke/songs/t/original.mp3',
    )
    expect(karaokePreviewUrl({ id: 't', title: 'T', audio: 'songs/t/minus.mp3' })).toBe(
      '/api/karaoke/songs/t/minus.mp3',
    )
    expect(karaokePreviewUrl({ id: 't', title: 'T' })).toBe('/api/karaoke/songs/t/minus.mp3')
  })

  it('keeps absolute embedded paths and remote URLs as is', () => {
    expect(
      karaokePreviewUrl({ id: 't', title: 'T', audio: '/api/karaoke/songs/t/minus.mp3' }),
    ).toBe('/api/karaoke/songs/t/minus.mp3')
    expect(karaokePreviewUrl({ id: 't', title: 'T', audio: 'https://cdn/x.mp3' })).toBe('https://cdn/x.mp3')
  })
})

describe('karaokePreviewTrack', () => {
  it('builds a ready player track without likes', () => {
    const t = karaokePreviewTrack({ id: 't', title: 'T', duration: 40, audio: 'songs/t/minus.mp3' })
    expect(t.id).toBe('karaoke:t')
    expect(t.url).toBe('/api/karaoke/songs/t/minus.mp3')
    expect(t.status).toBe('ready')
    expect(t.likes).toBe(0)
  })
})
