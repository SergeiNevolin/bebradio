/**
 * Voice-activity detector для кота-индикатора пения.
 *
 * Проблема абсолютных порогов по RMS: усиление микрофона, расстояние до него
 * и шум комнаты у всех разные — один и тот же порог для кого-то недостижим,
 * а у кого-то срабатывает на тишине. Поэтому работаем не с абсолютным
 * уровнем, а с превышением над адаптивным полом шума (SNR, дБ):
 * пол сам подстраивается под микрофон и комнату.
 *
 * Три идеи внутри:
 * 1. Пол шума — медленный минимум RMS: тишину догоняет за ~5 кадров,
 *    вверх ползёт еле-еле (tau ~8с), чтобы голос не утаскивал его за собой.
 * 2. Огибающая превышения: атака мгновенная (кот просыпается сразу),
 *    спад ~300мс — переживает глухие согласные и паузы между словами.
 * 3. Гейт по питчу: LivePitch уже считается в плеере раз в 120мс; хит
 *    с памятью 350мс подтверждает «это голос, а не стук/кашель».
 */

export type VoiceMode = 'silent' | 'soft' | 'loud'

/** Пороги превышения огибающей над полом, дБ (с гистерезисом). */
export const VAD_SOFT_ON_DB = 8
export const VAD_SOFT_OFF_DB = 5
export const VAD_LOUD_ON_DB = 16
export const VAD_LOUD_OFF_DB = 12

/** Сколько помним последний питч-хит, мс. */
export const VAD_VOICE_MEMORY_MS = 350

/** Спад огибающей за кадр (~60fps): tau ≈ 0.28с. */
const RELEASE_K = 0.06
/** Пол догоняет тишину быстро (~5 кадров), вверх — медленно (room drift). */
const FLOOR_DOWN_K = 0.3
const FLOOR_UP_K = 0.002

export interface VadState {
  /** Пол шума в единицах RMS; null — ещё ни одного кадра. */
  floor: number | null
  /** Огибающая превышения над полом, дБ (0..40). */
  envDb: number
  /** performance.now() последнего питч-хита. */
  lastVoiceMs: number
  /** Текущий режим (гистерезис опирается на него). */
  mode: VoiceMode
}

export function createVad(): VadState {
  return { floor: null, envDb: 0, lastVoiceMs: -Infinity, mode: 'silent' }
}

/** Скормить свежий RMS кадра: обновляет пол шума и огибающую. */
export function updateVadEnv(s: VadState, rms: number): void {
  const r = Math.max(0, rms)
  if (s.floor === null) {
    s.floor = r
  } else if (r < s.floor) {
    s.floor += (r - s.floor) * FLOOR_DOWN_K
  } else {
    s.floor += (r - s.floor) * FLOOR_UP_K
  }
  const db = Math.min(40, Math.max(0, 20 * Math.log10(r / (s.floor + 1e-6))))
  s.envDb = db > s.envDb ? db : s.envDb + (db - s.envDb) * RELEASE_K
}

/** Отметить хит детектора питча (это голос, а не шум). */
export function noteVadVoice(s: VadState, nowMs: number): void {
  s.lastVoiceMs = nowMs
}

/**
 * Текущий режим: без живого микрофона или без свежего питча — всегда
 * 'silent' (энергия без голоса = шум). Иначе — по огибающей с гистерезисом.
 * Результат запоминается в s.mode и возвращается.
 */
export function resolveVadMode(s: VadState, nowMs: number, live: boolean): VoiceMode {
  if (!live || nowMs - s.lastVoiceMs > VAD_VOICE_MEMORY_MS) {
    s.mode = 'silent'
    return s.mode
  }
  const db = s.envDb
  switch (s.mode) {
    case 'silent':
      s.mode = db >= VAD_LOUD_ON_DB ? 'loud' : db >= VAD_SOFT_ON_DB ? 'soft' : 'silent'
      break
    case 'soft':
      s.mode = db >= VAD_LOUD_ON_DB ? 'loud' : db < VAD_SOFT_OFF_DB ? 'silent' : 'soft'
      break
    case 'loud':
      s.mode = db < VAD_SOFT_OFF_DB ? 'silent' : db < VAD_LOUD_OFF_DB ? 'soft' : 'loud'
      break
  }
  return s.mode
}
