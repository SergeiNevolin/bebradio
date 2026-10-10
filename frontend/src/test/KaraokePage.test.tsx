import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../karaoke/App', () => ({ default: () => <div>KARAOKE_APP</div> }))

const playerBox: { current: { id: string } | null } = { current: null }
vi.mock('../context/PlayerContext', () => ({
  usePlayer: () => ({ player: { current: playerBox.current } }),
}))

import KaraokePage from '../pages/KaraokePage'
import { useKaraoke } from '../karaoke/store'

describe('KaraokePage', () => {
  beforeEach(() => {
    playerBox.current = null
    useKaraoke.getState().setSoundActive(false)
  })

  it('монтирует караоке нативно на всю область под навбаром, без iframe', () => {
    const { container } = render(<KaraokePage />)
    expect(screen.getByText('KARAOKE_APP')).toBeTruthy()
    expect(container.querySelector('iframe')).toBeNull()
    expect(container.querySelector('.karaoke-root')).not.toBeNull()
  })

  it('резервирует полосу только под видимую плашку плеера', () => {
    // Нет текущего трека — плашки нет, резервировать нечего.
    const first = render(<KaraokePage />)
    expect(first.container.querySelector('.karaoke-root')?.className).not.toContain('barClear')
    first.unmount()

    // Трек есть, песня не поётся — плашка видна, место нужно.
    playerBox.current = { id: 'm1' }
    const second = render(<KaraokePage />)
    const root = () => second.container.querySelector('.karaoke-root')?.className ?? ''
    expect(root()).toContain('barClear')

    // Песня поётся — плеер спрятан вместе с плашкой.
    act(() => useKaraoke.getState().setSoundActive(true))
    expect(root()).not.toContain('barClear')
  })

  it('тянет полосу на весь вьюпорт пока поётся (сайдбар спрятан)', () => {
    const { container } = render(<KaraokePage />)
    const wrap = () => container.firstElementChild?.className ?? ''
    expect(wrap()).not.toContain('wrapFull')
    act(() => useKaraoke.getState().setSoundActive(true))
    expect(wrap()).toContain('wrapFull')
  })
})
