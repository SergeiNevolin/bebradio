/**
 * Кот-индикатор пения: простая обработка громкости микрофона.
 *
 * Работает ровно как индикация уровня в шапке плеера (Player: setLevel
 * с масштабом MIC_LEVEL_SCALE): один и тот же кадр микрофона превращается
 * в уровень 0..1, кот лишь решает, какой картинкой его показывать.
 * Никакого адаптивного пола шума, питч-гейтов и нейросетей.
 *
 * Три правила:
 * 1. Сглаженный уровень ниже тишины (или тишина, или выключенный/пауза) — кот молчит.
 * 2. Сглаженный уровень выше порога громко — кот поёт в полный голос.
 * 3. Между ними или в тень после последнего звука (до HOLD) — кот поёт тихо:
 *    так переживаются паузы между словами и глухие согласные.
 */

export type VoiceMode = 'silent' | 'soft' | 'loud'

/**
 * Масштаб индикатора уровня: level = min(1, rms * MIC_LEVEL_SCALE).
 * Общий для полоски уровня в шапке и для кота, чтобы они показывали одно и то же.
 */
export const MIC_LEVEL_SCALE = 6

/** Ниже этого уровня (в единицах 0..1) звук считается тишиной. */
export const CAT_QUIET_LEVEL = 0.15
/** Выше этого уровня кот поёт «громко». */
export const CAT_LOUD_LEVEL = 0.55

/** Сколько мс сглаженный уровень может держаться ниже тишинного — между словами. */
export const CAT_SILENCE_HOLD_MS = 250

/** Спад сглаживания уровня за кадр (~60fps): tau ≈ 0.17с. Атака — мгновенная. */
const RELEASE_K = 0.1

export interface VoiceState {
  /** Сглаженный уровень микрофона 0..1 (шкала индикации). */
  env: number
  /** performance.now() последнего кадра, где сглаженный уровень ≥ тишины. */
  lastActiveMs: number
  mode: VoiceMode
}

export function createVoiceState(): VoiceState {
  return { env: 0, lastActiveMs: -Infinity, mode: 'silent' }
}

/** воспринять кадр уровня 0..1 (та же шкала, что и индикатор уровня). */
export function noteVoiceLevel(s: VoiceState, level: number, nowMs: number): void {
  const v = Math.max(0, Math.min(1, level))
  s.env = v > s.env ? v : s.env + (v - s.env) * RELEASE_K
  if (s.env >= CAT_QUIET_LEVEL) s.lastActiveMs = nowMs
}

/**
 * Режим кота: без живого микрофона — всегда silent. Иначе три состояния
 * по сглаженному уровню с удержанием между словами.
 */
export function resolveVoiceMode(s: VoiceState, nowMs: number, live: boolean): VoiceMode {
  if (!live || nowMs - s.lastActiveMs > CAT_SILENCE_HOLD_MS) {
    s.mode = 'silent'
    return s.mode
  }
  s.mode = s.env >= CAT_LOUD_LEVEL ? 'loud' : 'soft'
  return s.mode
}
