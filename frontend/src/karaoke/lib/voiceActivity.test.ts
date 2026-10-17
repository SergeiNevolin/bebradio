import { describe, expect, it } from 'vitest'
import {
  CAT_LOUD_LEVEL,
  CAT_QUIET_LEVEL,
  CAT_SILENCE_HOLD_MS,
  createVoiceState,
  MIC_LEVEL_SCALE,
  noteVoiceLevel,
  resolveVoiceMode,
} from './voiceActivity'

const NOW = 10_000

describe('voiceActivity: уровень', () => {
  it('атака мгновенная, спад плавный', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, 0.5, NOW)
    expect(s.env).toBe(0.5)
    for (let i = 1; i <= 5; i++) noteVoiceLevel(s, 0, NOW + i) // ~5 кадров тишины
    expect(s.env).toBeGreaterThan(0.2)
    expect(s.env).toBeLessThan(0.5)
  })

  it('уровень ограничен шкалой 0..1', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, 5, NOW)
    expect(s.env).toBe(1)
    noteVoiceLevel(s, -1, NOW + 1)
    expect(s.env).toBeLessThan(1)
  })

  it('тот же масштаб, что и индикатор уровня в шапке', () => {
    expect(MIC_LEVEL_SCALE).toBe(6)
  })
})

describe('voiceActivity: режимы', () => {
  it('нет звука / тихий звук — кот молчит', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, CAT_QUIET_LEVEL - 0.05, NOW)
    expect(resolveVoiceMode(s, NOW, true)).toBe('silent')
  })

  it('средняя громкость — тихо поёт', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, 0.3, NOW)
    expect(resolveVoiceMode(s, NOW, true)).toBe('soft')
  })

  it('громкость выше порога — поёт в полный голос', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, 1, NOW)
    expect(s.env).toBeGreaterThanOrEqual(CAT_LOUD_LEVEL)
    expect(resolveVoiceMode(s, NOW, true)).toBe('loud')
  })

  it('без живого микрофона — всегда silent', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, 1, NOW)
    expect(resolveVoiceMode(s, NOW, false)).toBe('silent')
  })
})

describe('voiceActivity: переживание паузы', () => {
  it('короткая пауза между словами не гасит кота', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, 0.3, NOW)
    expect(resolveVoiceMode(s, NOW + CAT_SILENCE_HOLD_MS - 1, true)).toBe('soft')
  })

  it('долгая тишина гасит кота', () => {
    const s = createVoiceState()
    noteVoiceLevel(s, 0.3, NOW)
    expect(resolveVoiceMode(s, NOW + CAT_SILENCE_HOLD_MS + 1, true)).toBe('silent')
  })

  it('сглаженный уровень ниже тишинного молчит после удержания', () => {
    // был громкий кадр, затем уровень ушёл вниз: env спадает, кадры перестают
    // продлевать lastActiveMs; пока идёт удержание — тихо поёт, потом гаснет
    const s = createVoiceState()
    noteVoiceLevel(s, 0.5, NOW)
    for (let i = 1; i <= 40; i++) noteVoiceLevel(s, 0.05, NOW + i)
    expect(s.env).toBeLessThan(CAT_QUIET_LEVEL)
    expect(s.lastActiveMs).toBeLessThan(NOW + 20)
    expect(resolveVoiceMode(s, NOW + 30, true)).toBe('soft')
    expect(resolveVoiceMode(s, s.lastActiveMs + CAT_SILENCE_HOLD_MS + 1, true)).toBe('silent')
  })
})
