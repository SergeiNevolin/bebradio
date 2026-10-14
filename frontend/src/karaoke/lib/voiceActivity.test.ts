import { describe, expect, it } from 'vitest'
import {
  createVad,
  noteVadVoice,
  resolveVadMode,
  updateVadEnv,
  VAD_LOUD_OFF_DB,
  VAD_LOUD_ON_DB,
  VAD_SOFT_OFF_DB,
  VAD_SOFT_ON_DB,
  VAD_VOICE_MEMORY_MS,
  type VadState,
} from './voiceActivity'

const NOW = 10_000

/** состояние с готовым полом, заданной огибающей и свежим голосом */
function voiced(envDb: number, mode: VadState['mode'] = 'silent'): VadState {
  return { ...createVad(), floor: 0.01, envDb, lastVoiceMs: NOW, mode }
}

function feed(s: VadState, rms: number, n: number): void {
  for (let i = 0; i < n; i++) updateVadEnv(s, rms)
}

describe('voiceActivity: пол шума', () => {
  it('первый кадр задаёт пол', () => {
    const s = createVad()
    expect(s.floor).toBeNull()
    updateVadEnv(s, 0.02)
    expect(s.floor).toBeCloseTo(0.02, 6)
  })

  it('тишину догоняет быстро, вверх ползёт медленно', () => {
    const s = createVad()
    feed(s, 0.2, 5) // громко — пол подтянулся вверх, но недалеко
    const afterLoud = s.floor ?? 0
    feed(s, 0.01, 10) // тишина — пол падает быстро
    expect(s.floor ?? 1).toBeLessThan(0.03)
    // а вверх за те же 10 кадров почти не двинулся бы: проверяем медленно
    const s2 = createVad()
    feed(s2, 0.01, 5)
    feed(s2, 0.2, 10)
    expect(s2.floor ?? 1).toBeLessThan(0.05)
    expect(afterLoud).toBeGreaterThan(0)
  })
})

describe('voiceActivity: огибающая', () => {
  it('атака мгновенная', () => {
    const s = createVad()
    feed(s, 0.01, 20) // тишина: пол и огибающая осели
    const quiet = s.envDb
    updateVadEnv(s, 0.2) // всплеск — один кадр
    expect(s.envDb).toBeGreaterThan(quiet + 10)
  })

  it('спад медленный: переживает паузы между словами', () => {
    const s = createVad()
    feed(s, 0.01, 20) // тишина: пол калибруется
    feed(s, 0.2, 5) // всплеск голоса
    const peak = s.envDb
    expect(peak).toBeGreaterThan(10)
    feed(s, 0.005, 10) // ~170мс тишины
    expect(s.envDb).toBeGreaterThan(peak * 0.4)
    expect(s.envDb).toBeLessThan(peak)
  })
})

describe('voiceActivity: режимы', () => {
  it('без голоса — всегда silent, даже при энергии', () => {
    const s = voiced(30)
    s.lastVoiceMs = NOW - VAD_VOICE_MEMORY_MS - 1 // питч протух
    expect(resolveVadMode(s, NOW, true)).toBe('silent')
  })

  it('без живого микрофона — всегда silent', () => {
    const s = voiced(30)
    noteVadVoice(s, NOW)
    expect(resolveVadMode(s, NOW, false)).toBe('silent')
  })

  it('протухший питч гасит кота', () => {
    const s = voiced(30)
    s.lastVoiceMs = NOW - VAD_VOICE_MEMORY_MS - 1
    expect(resolveVadMode(s, NOW, true)).toBe('silent')
  })

  it('пороги: тихо и громко', () => {
    expect(resolveVadMode(voiced(VAD_SOFT_ON_DB - 1), NOW, true)).toBe('silent')
    expect(resolveVadMode(voiced(VAD_SOFT_ON_DB), NOW, true)).toBe('soft')
    expect(resolveVadMode(voiced(VAD_LOUD_ON_DB), NOW, true)).toBe('loud')
  })

  it('гистерезис держит режим внутри вилки', () => {
    const loud = voiced((VAD_LOUD_ON_DB + VAD_LOUD_OFF_DB) / 2, 'loud')
    expect(resolveVadMode(loud, NOW, true)).toBe('loud')
    const soft = voiced((VAD_SOFT_ON_DB + VAD_SOFT_OFF_DB) / 2, 'soft')
    expect(resolveVadMode(soft, NOW, true)).toBe('soft')
  })

  it('падение ниже вилки спускает вниз', () => {
    const loud = voiced(VAD_LOUD_OFF_DB - 1, 'loud')
    expect(resolveVadMode(loud, NOW, true)).toBe('soft')
    const soft = voiced(VAD_SOFT_OFF_DB - 0.5, 'soft')
    expect(resolveVadMode(soft, NOW, true)).toBe('silent')
  })
})
