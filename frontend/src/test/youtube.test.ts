import { describe, it, expect } from 'vitest'
import { youtubeId } from '../lib/youtube'

describe('youtubeId', () => {
  it('reads the id from a watch URL', () => {
    expect(youtubeId('https://www.youtube.com/watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
  })

  it('reads the id from a watch URL with extra params', () => {
    expect(youtubeId('https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=RDx&t=30')).toBe('dQw4w9WgXcQ')
  })

  it('reads the id from a youtu.be short link', () => {
    expect(youtubeId('https://youtu.be/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
  })

  it('reads the id from shorts and embed URLs', () => {
    expect(youtubeId('https://www.youtube.com/shorts/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
    expect(youtubeId('https://www.youtube.com/embed/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
  })

  it('returns an empty string for non-YouTube or missing input', () => {
    expect(youtubeId('https://example.com/song.mp3')).toBe('')
    expect(youtubeId('')).toBe('')
    expect(youtubeId(null)).toBe('')
    expect(youtubeId(undefined)).toBe('')
  })
})
