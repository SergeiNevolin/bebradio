import { AnimatePresence, motion } from 'framer-motion'
import { ArrowLeft, Loader2, Pause, Play, RotateCcw, Volume2, Mic, MicOff } from 'lucide-react'
import { memo, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { formatTime, loadSong, clearLocalLyrics, clearLocalSkips, inSkip, wantOriginal } from '../lib/songs'
import { warmSongAudio } from '../lib/audioCache'
import { ballProgress, ballXY, karaokeFrame, lineTextSize } from '../lib/karaoke'
import { gradeText, LivePitch, makeRefLookup, micLag, pitchClassError, sampleFrame, PERFECT_ERR, GOOD_ERR } from '../lib/pitch'
import { buildSongScore, extractNotes, scoreNotes, scoreTiming, scoreWords, applySkips } from '../lib/score'
import type { ScoreResult, Segment, SkipRange } from '../lib/types'
import { useKaraoke } from '../store'
import PitchStrip, { type UserDot } from './PitchStrip'
import Editor from './Editor'
import SingingCat from './SingingCat'
import { createVad, noteVadVoice, resolveVadMode, updateVadEnv, type VoiceMode } from '../lib/voiceActivity'

type Phase = 'ready' | 'countdown' | 'playing' | 'paused' | 'finished'

interface Frame {
  t: number
  user: number | null
  ref: number
}

interface TopBarProps {
  title: string
  /** секунды (не мс): шапка обновляется раз в секунду, а не каждый тик */
  timeSec: number
  duration: number
  withMic: boolean
  micLive: boolean
  level: number
  onBack: () => void
}

/** шапка плеера: статична между тиками звука */
const TopBar = memo(function TopBar({ title, timeSec, duration, withMic, micLive, level, onBack }: TopBarProps) {
  return (
    <div className="flex items-center gap-2.5 py-3">
      <button onClick={onBack} title="Вернуться в каталог" aria-label="Вернуться в каталог"
        className="flex h-10 shrink-0 items-center gap-1.5 rounded-full bg-surface-hover px-3.5 text-[13px] font-medium text-text transition hover:bg-border hover:text-text max-sm:hidden">
        <ArrowLeft className="h-4 w-4" />
        <span className="hidden sm:inline">Каталог</span>
      </button>
      <div className="min-w-0 flex-1">
        <div className="truncate text-[15px] font-medium text-text">{title}</div>
        <div className="text-xs text-muted tabular-nums">{formatTime(timeSec)} / {formatTime(duration)}</div>
      </div>
      <div title={withMic ? 'Микрофон включён' : 'Микрофон выключен — поёте без оценки'}
        className={`flex shrink-0 items-center gap-1.5 rounded-full px-3 py-2 text-xs ${micLive ? 'bg-surface-hover text-text' : 'bg-surface text-muted/70'}`}>
        {withMic ? <Mic className="h-3.5 w-3.5" /> : <MicOff className="h-3.5 w-3.5" />}
        <span className="hidden font-medium md:inline">{withMic ? 'Микрофон' : 'Без микрофона'}</span>
        <div className="h-1 w-12 overflow-hidden rounded-full bg-border">
          <div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${Math.round(level * 100)}%` }} />
        </div>
      </div>
    </div>
  )
})

interface ControlsBarProps {
  phase: Phase
  volume: number
  onVolume: (v: number) => void
  backing: 'minus' | 'full'
  hasOriginal: boolean
  onToggle: () => void
  onRestart: () => void
  onBacking: (b: 'minus' | 'full') => void
}

/** управление: перерисовывается только по действиям, не по тикам */
const ControlsBar = memo(function ControlsBar({ phase, volume, onVolume, backing, hasOriginal, onToggle, onRestart, onBacking }: ControlsBarProps) {
  return (
    <div className="flex items-center gap-2 py-2 sm:gap-3 sm:py-4">
      {(phase === 'playing' || phase === 'paused') && (
        <>
          <button onClick={onToggle} title={phase === 'playing' ? 'Пауза (пробел)' : 'Продолжить (пробел)'}
            aria-label={phase === 'playing' ? 'Пауза' : 'Продолжить'}
            className="grid h-10 w-10 place-items-center rounded-full bg-primary text-white transition hover:bg-primary-hover sm:h-12 sm:w-12">
            {phase === 'playing' ? <Pause className="h-5 w-5" /> : <Play className="ml-0.5 h-5 w-5" />}
          </button>
          <button onClick={onRestart} title="Начать песню сначала"
            aria-label="Начать песню сначала"
            className="flex h-10 items-center gap-1.5 rounded-full bg-surface-hover px-3.5 text-[13px] font-medium text-text transition hover:bg-border hover:text-text max-sm:hidden">
            <RotateCcw className="h-4 w-4" />
            Сначала
          </button>
        </>
      )}
      <div className="flex flex-1 items-center gap-2">
        <Volume2 className="h-4 w-4 shrink-0 text-muted" />
        <input type="range" min={0} max={1} step={0.01} value={volume}
          onChange={(e) => onVolume(Number(e.target.value))}
          className="w-full max-w-36 accent-primary" aria-label="Громкость" />
      </div>
      {hasOriginal && (
        <div className="flex shrink-0 overflow-hidden rounded-full bg-surface-hover"
          title="Минус — петь самому · Плюс — оригинал с вокалом">
          {(['minus', 'full'] as const).map((b) => (
            <button key={b} onClick={() => onBacking(b)}
              className={`px-3.5 py-2 text-[13px] font-medium transition ${backing === b ? 'bg-primary font-semibold text-white' : 'text-muted hover:text-text'}`}>
              {b === 'minus' ? 'Минус' : 'Плюс'}
            </button>
          ))}
        </div>
      )}
      {phase === 'finished' && (
        <button onClick={onRestart}
          className="rounded-full bg-primary px-5 py-2.5 text-sm font-semibold text-white transition hover:bg-primary-hover">
          Ещё раз
        </button>
      )}
    </div>
  )
})

interface NextLine {
  i: number
  text: string
  part?: string
  showPart: boolean
}

interface CurrentLineProps {
  s: Segment
  idx: number
  segs: Segment[]
  sungNow: number
  time: number
  ballOn: boolean
  centers: number[]
  lineRef: React.RefObject<HTMLDivElement>
  wordRefs: React.MutableRefObject<Array<HTMLSpanElement | null>>
}

/** крупная строка: шарик + пословная подсветка (обновляется каждый тик) */
function CurrentLine({ s, idx, segs, sungNow, time, ballOn, centers, lineRef, wordRefs, skips }: CurrentLineProps & { skips: SkipRange[] }) {
  const nWords = s.words.length
  const ready = ballOn && centers.length === nWords
  const sung = Math.max(0, Math.min(sungNow, nWords))
  const p = ballProgress(s, sung, time)
  const { x: ballX, y: ballY } = ballXY(centers, sung, p)
  const showPart = !!s.part && (idx === 0 || segs[idx - 1]?.part !== s.part)
  return (
    <div>
      {showPart && (
        <div className="mb-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-primary/70">
          {s.part}
        </div>
      )}
      <div ref={lineRef} className="relative pt-8">
        {ready && (
          <div className="pointer-events-none absolute z-10" style={{ left: ballX, top: 4, transform: `translate(-50%, ${ballY}px)` }}>
            <div className="h-3.5 w-3.5 rounded-full bg-primary shadow-[0_0_12px_2px_var(--color-primary)]" />
          </div>
        )}
                  <p className={lineTextSize(nWords)}>
          {s.words.map((w, j) => {
            const skipped = inSkip(skips, (w.s + w.e) / 2)
            return (
              <span key={j} ref={(el) => { wordRefs.current[j] = el }} className={skipped ? 'text-muted/70 line-through decoration-zinc-700' : j < sung ? 'text-primary' : 'text-text'}>
                {w.w}{' '}
              </span>
            )
          })}
        </p>
      </div>
    </div>
  )
}

/** следующие строки: статичны, пока не сменилось окно */
const NextLines = memo(function NextLines({ lines }: { lines: NextLine[] }) {
  return (
    <>
      {lines.map((l) => (
        <div key={`nx-${l.i}`}>
          {l.showPart && (
            <div className="mb-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted/70">
              {l.part}
            </div>
          )}
          <p className="text-2xl leading-relaxed text-muted sm:text-3xl">
            {l.text}
          </p>
        </div>
      ))}
    </>
  )
})

export default function Player() {
  const song = useKaraoke((s) => s.song)
  const openSong = useKaraoke((s) => s.openSong)
  const back = useKaraoke((s) => s.back)
  const pushRecent = useKaraoke((s) => s.pushRecent)
  const setSoundActive = useKaraoke((s) => s.setSoundActive)

  const [phase, setPhase] = useState<Phase>('ready')
  const [count, setCount] = useState(3)
  const [time, setTime] = useState(0)
  const [duration, setDuration] = useState(song?.duration ?? 0)
  // подложка: минус (петь самому) или плюс (оригинал с вокалом)
  const [backing, setBacking] = useState<'minus' | 'full'>('minus')
  // при смене подложки держим позицию и состояние игры
  const keepTime = useRef<number | null>(null)
  const autoPlay = useRef(false)
  const countTimer = useRef(0)
  const phaseRef = useRef(phase)
  phaseRef.current = phase
  const [volume, setVolume] = useState(0.85)
  const [withMic, setWithMic] = useState(true)
  const [level, setLevel] = useState(0)
  const [micError, setMicError] = useState<string | null>(null)
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])
  const [deviceId, setDeviceId] = useState<string>('')
  const [score, setScore] = useState<ScoreResult | null>(null)
  const [scored, setScored] = useState(false)
  const editorOpen = useKaraoke((s) => s.editorOpen)
  // При пении таб-бар скрыт (см. App) — занимаем всю высоту до самого низа.
  // Вне пения оставляем запас 64px под нижнюю навигацию.
  const singing = useKaraoke((s) => s.soundActive)
  const setEditorOpen = useKaraoke((s) => s.setEditorOpen)
  // счётчик сбросов: заставляет открытый редактор перечитать song.segments
  const [editorReset, setEditorReset] = useState(0)
  // позиция пальца на ползунке во время драга — чтобы не дёргался за звуком
  const [scrub, setScrub] = useState<number | null>(null)
  // Кот-индикатор: VAD-режим обновляется прямо в rAF-кадре loop() —
  // без таймеров и useEffect. Показываем только при живом микрофоне,
  // иначе (пауза/старт/финал, замороженный level) — всегда молчит.
  const [catMode, setCatMode] = useState<VoiceMode>('silent')
  const vadRef = useRef(createVad())
  const withMicRef = useRef(withMic)
  withMicRef.current = withMic
  const micLive = withMic && phase === 'playing'
  // ждём байты с сети (seek в непрогретое место): показываем «Буферизация…»
  const [waiting, setWaiting] = useState(false)

  const audioRef = useRef<HTMLAudioElement>(null)
  /** вторая дорожка (оригинал с вокалом): глушим/даём звук на пропусках «не поём» */
  const origRef = useRef<HTMLAudioElement>(null)
  /** какая дорожка сейчас слышна: оригинал или минус */
  const useOrig = useRef(false)
  /** последний ресинк дрифта (performance.now): не дёргаем seek каждый тик */
  const lastSyncRef = useRef(0)
  /** blob-URL, догревшийся посреди пения слышимой дорожки: применим на паузе/seek/рестарте */
  const pendingBlob = useRef<{ minus?: string; orig?: string }>({})
  const rafRef = useRef(0)
  const streamRef = useRef<MediaStream | null>(null)
  const ctxRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const waveRef = useRef<Float32Array>(new Float32Array(2048))
  const detectorRef = useRef(new LivePitch())
  const smoothRef = useRef<number[]>([])
  const lookup = useMemo(() => makeRefLookup(song?.pitchSmooth ?? song?.pitch ?? null), [song])
  const framesRef = useRef<Frame[]>([])
  const dotsRef = useRef<UserDot[]>([])
  const lastPitchRef = useRef(0)
  // Шарик караоке: refs/состояние; измерение — ниже, после activeSeg
  const lineRef = useRef<HTMLDivElement>(null)
  const wordRefs = useRef<Array<HTMLSpanElement | null>>([])
  const [centers, setCenters] = useState<number[]>([])
  const [measureKey, setMeasureKey] = useState(0)

  const stopMic = useCallback(() => {
    cancelAnimationFrame(rafRef.current)
    streamRef.current?.getTracks().forEach((t) => t.stop())
    streamRef.current = null
    void ctxRef.current?.close().catch(() => undefined)
    ctxRef.current = null
    analyserRef.current = null
  }, [])

  useEffect(() => stopMic, [stopMic])
  // размонтирование посреди отсчёта — таймер снять
  useEffect(() => () => window.clearInterval(countTimer.current), [])
  // ушли со страницы во время пения — вернуть общий плеер
  useEffect(() => () => setSoundActive(false), [setSoundActive])
  useEffect(() => {
    const a = audioRef.current
    if (a) a.volume = volume
    const o = origRef.current
    if (o) o.volume = volume
  }, [volume])

  const skips = useMemo(() => song?.skips ?? [], [song])

  /** какую дорожку должно быть слышно в момент t (оригинал — в плюсе и на пропусках) */
  const wantOrigAt = useCallback((t: number): boolean => (
    wantOriginal(backing, skips, !!song?.original, t)
  ), [backing, song, skips])

  /** переключить слышимую дорожку; оригинал ресинкаем к мастер-часам минуса.
   *  Стабильность важнее идеального совпадения: лишний seek = щелчок/дырка,
   *  поэтому мотаем цель только если реально разъехались (>120мс) и она не
   *  в середине seek/буферизации. Размьюченная, но вставшая на паузу дорожка —
   *  это тишина: такую явно дожимаем play(). */
  const flipAudible = useCallback((want: boolean) => {
    const a = audioRef.current
    const o = origRef.current
    if (!a || !o || !song?.original) return
    if (want === useOrig.current) return
    try {
      if (Number.isFinite(a.currentTime) && Number.isFinite(o.currentTime) && !o.seeking) {
        const diff = Math.abs(o.currentTime - a.currentTime)
        if (diff > 0.12 && o.readyState >= 1) o.currentTime = a.currentTime
      }
    } catch {
      /* ignore */
    }
    if (phaseRef.current === 'playing') {
      const target = want ? o : a
      if (target.paused) {
        void target.play().catch(() => undefined)
      }
    }
    o.muted = !want
    a.muted = want
    useOrig.current = want
  }, [song])

  // границы пропусков + дрейф оригинала: тик времени всё чинит.
  // Два независимых <audio> всегда чуть дрейфуют — дёргать seek на каждые
  // 150мс значит получить то самое «прерывисто» в плюсе. Поэтому дозор редкий
  // (не чаще раза в 1.5с), с высоким порогом (300мс) и только когда оригинал
  // реально играет и готов (не seeking, есть данные).
  useEffect(() => {
    if (phaseRef.current !== 'playing' && phaseRef.current !== 'paused') return
    flipAudible(wantOrigAt(time))
    const a = audioRef.current
    const o = origRef.current
    if (a && o && useOrig.current && !o.paused && !o.seeking && o.readyState >= 3) {
      const drift = Math.abs(o.currentTime - a.currentTime)
      const now = performance.now()
      if (drift > 0.3 && now - lastSyncRef.current > 1500) {
        lastSyncRef.current = now
        try {
          o.currentTime = a.currentTime
        } catch {
          /* ignore */
        }
      }
    }
  }, [time, backing, song, skips, flipAudible, wantOrigAt])

  // караоке играет строго по времени: отсортированная копия
  // (в файле порядок свободный — как расставил редактор)
  const segsSorted = useMemo(() => [...(song?.segments ?? [])].sort((a, b) => a.start - b.start), [song])
  const frame = karaokeFrame(segsSorted, time)
  const winStart = frame.winStart
  // следующие строки — мемоизированы (строго до ранних return: хуки всегда в одном порядке)
  const nextLines = useMemo(
    () => {
      const sg = song?.segments ?? []
      return [winStart + 1, winStart + 2]
        .map((i) => ({ s: sg[i] as Segment | undefined, i }))
        .filter((v): v is { s: Segment; i: number } => !!v.s)
        .map((v) => ({
          i: v.i,
          text: v.s.text,
          part: v.s.part,
          showPart: !!v.s.part && (v.i === 0 || sg[v.i - 1]?.part !== v.s.part),
        }))
    },
    [winStart, song],
  )

  // Перемеряем центры слов при смене строки / ресайзе / загрузке шрифтов
  useLayoutEffect(() => {
    const box = lineRef.current
    if (!box) return
    const cRect = box.getBoundingClientRect()
    const pts: number[] = []
    for (let j = 0; j < wordRefs.current.length; j++) {
      const el = wordRefs.current[j]
      if (!el) break
      const r = el.getBoundingClientRect()
      pts.push(r.left - cRect.left + r.width / 2)
    }
    setCenters(pts)
  }, [winStart, song, measureKey])

  useEffect(() => {
    const bump = () => setMeasureKey((k) => k + 1)
    window.addEventListener('resize', bump)
    const fontsReady = document.fonts?.ready
    if (fontsReady) fontsReady.then(bump).catch(() => undefined)
    return () => window.removeEventListener('resize', bump)
  }, [])

  const loop = useCallback(() => {
    const a = audioRef.current
    if (!a) return // размонтировано — цепь останавливаем
    try {
      const t = a.currentTime
      // мс-точность подсветки: тики 10мс, тяжёлое отрисовывается редко (см. memo ниже)
      setTime((prev) => (Math.abs(t - prev) < 0.01 ? prev : t))

      const an = analyserRef.current
      const ctx = ctxRef.current
      if (an && ctx) {
        const buf = waveRef.current
        an.getFloatTimeDomainData(buf as Float32Array<ArrayBuffer>)
        let sum = 0
        for (let i = 0; i < buf.length; i++) sum += buf[i] * buf[i]
      const rms = Math.sqrt(sum / buf.length)
      setLevel((prev) => {
        const v = Math.min(1, rms * 6)
        return Math.abs(v - prev) < 0.03 ? prev : v
      })
        // VAD кота: пол шума + огибающая каждый кадр, питч — гейт «это голос»
        const vad = vadRef.current
        updateVadEnv(vad, rms)
        const now = performance.now()
        if (now - lastPitchRef.current > 120) {
          lastPitchRef.current = now
          const hit = detectorRef.current.detect(buf, ctx.sampleRate)
          if (hit) noteVadVoice(vad, now)
          const fr = sampleFrame(
            smoothRef.current,
            t,
            hit ? hit.freq : null,
            lookup,
            micLag(ctx.sampleRate),
            hit ? hit.clarity : 1,
          )
          if (fr && fr.user !== null) {
            const err = pitchClassError(fr.user, fr.ref)
            const perfect = err <= PERFECT_ERR
            framesRef.current.push(fr)
            dotsRef.current.push({ t, midi: fr.user, hit: err <= GOOD_ERR, perfect })
            if (dotsRef.current.length > 3000) dotsRef.current.shift()
          }
        }
        // режим кота — каждый кадр, setState только при смене (дешево)
        const vadBefore = vad.mode
        resolveVadMode(vad, now, withMicRef.current && phaseRef.current === 'playing')
        if (vad.mode !== vadBefore) setCatMode(vad.mode)
      }
    } catch {
      // один плохой кадр (микрофон/питч) не убивает часы караоке
    } finally {
      // цепь кадров — всегда, иначе время встанет, а звук пойдёт дальше
      rafRef.current = requestAnimationFrame(loop)
    }
  }, [lookup])

  const startLoop = useCallback(() => {
    cancelAnimationFrame(rafRef.current)
    rafRef.current = requestAnimationFrame(loop)
  }, [loop])

  const setupMic = useCallback(async (): Promise<boolean> => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          deviceId: deviceId ? { exact: deviceId } : undefined,
          echoCancellation: true,
          noiseSuppression: true,
        },
      })
      streamRef.current = stream
      const Ctx = window.AudioContext
      const ctx = new Ctx()
      ctxRef.current = ctx
      const src = ctx.createMediaStreamSource(stream)
      const an = ctx.createAnalyser()
      an.fftSize = 2048
      src.connect(an)
      analyserRef.current = an
      const devs = await navigator.mediaDevices.enumerateDevices()
      setDevices(devs.filter((d) => d.kind === 'audioinput'))
      return true
    } catch {
      setMicError('Не удалось включить микрофон — можно петь без оценки')
      return false
    }
  }, [deviceId])

  const begin = useCallback(
    async (useMic: boolean) => {
      if (!audioRef.current || !song) return
      if (phaseRef.current !== 'ready') return // двойной клик по «Петь» — второй игнор
      pushRecent(song.id)
      setSoundActive(true)
      setMicError(null)
      setScore(null)
      setScored(false)
      framesRef.current = []
      dotsRef.current = []
      smoothRef.current = []
      vadRef.current = createVad()
      setCatMode('silent')
      setTime(0)
      audioRef.current.currentTime = 0
      // старт всегда с минуса; тик-эффект сам даст оригинал, если надо (плюс/пропуск)
      useOrig.current = false
      if (origRef.current) {
        origRef.current.muted = true
        try {
          origRef.current.currentTime = 0
        } catch {
          /* ignore */
        }
      }
      audioRef.current.muted = false
      setWithMic(useMic)
      if (useMic) {
        const ok = await setupMic()
        if (!ok) setWithMic(false)
      }
      setPhase('countdown')
      setCount(3)
      let n = 3
      window.clearInterval(countTimer.current)
      countTimer.current = window.setInterval(() => {
        n -= 1
        if (n <= 0) {
          window.clearInterval(countTimer.current)
          // оригинал всегда играет рядом приглушённым — переходы без пауз
          if (origRef.current) {
            try {
              origRef.current.currentTime = audioRef.current?.currentTime ?? 0
            } catch {
              /* ignore */
            }
            void origRef.current.play().catch(() => undefined)
          }
          audioRef.current
            ?.play()
            .then(() => {
              setPhase('playing')
              startLoop()
            })
            .catch((e: unknown) => {
              // Подмена src на blob могла оборвать этот play (AbortError) —
              // это не блок автоплея: дожимаем один раз.
              if ((e as Error | null)?.name === 'AbortError' && audioRef.current) {
                void audioRef.current
                  .play()
                  .then(() => {
                    setPhase('playing')
                    startLoop()
                  })
                  .catch(() => {
                    setPhase('ready')
                    setSoundActive(false)
                    setMicError('Браузер заблокировал автоплей — нажмите «Петь» ещё раз')
                  })
                return
              }
              setPhase('ready')
              setSoundActive(false)
              setMicError('Браузер заблокировал автоплей — нажмите «Петь» ещё раз')
            })
        } else setCount(n)
      }, 750)
    },
    [setupMic, song, startLoop, pushRecent, setSoundActive],
  )

  const togglePlay = useCallback(() => {
    const a = audioRef.current
    if (!a) return
    if (phase === 'playing') {
      a.pause()
      origRef.current?.pause()
      // пока стоим — беззвучно подбираем догревшиеся blob (streаm → память),
      // чтобы следующий старт уже шёл из памяти без сети
      const p = pendingBlob.current
      if (p.minus && a.getAttribute('src') !== p.minus) {
        const t = Number.isFinite(a.currentTime) ? a.currentTime : 0
        a.src = p.minus
        try { a.currentTime = t } catch { /* ignore */ }
      }
      const o0 = origRef.current
      if (o0 && p.orig && o0.getAttribute('src') !== p.orig) {
        const t = Number.isFinite(o0.currentTime) ? o0.currentTime : 0
        o0.src = p.orig
        try { o0.currentTime = t } catch { /* ignore */ }
      }
      pendingBlob.current = {}
      setPhase('paused')
      smoothRef.current = []
      cancelAnimationFrame(rafRef.current)
    } else if (phase === 'paused') {
      smoothRef.current = []
      const o = origRef.current
      if (o) {
        try {
          if (Number.isFinite(a.currentTime) && Number.isFinite(o.currentTime)
            && !o.seeking && Math.abs(o.currentTime - a.currentTime) > 0.12 && o.readyState >= 1) {
            o.currentTime = a.currentTime
          }
        } catch {
          /* ignore */
        }
        void o.play().catch(() => undefined)
      }
      void a.play().then(() => {
        setPhase('playing')
        startLoop()
      }).catch(() => {
        setPhase('paused')
      })
    }
  }, [phase, startLoop])

  /** переключить минус/плюс: сами дорожки не трогаем, тик-эффект переключит звук */
  const switchBacking = useCallback((b: 'minus' | 'full') => {
    if (b === backing) return
    setBacking(b)
    smoothRef.current = []
  }, [backing])

  /** смена src: вернуть позицию, продолжить если играло */
  const onBackingLoaded = useCallback((el: HTMLAudioElement) => {
    setDuration(el.duration || song?.duration || 0)
    if (keepTime.current != null) {
      try {
        el.currentTime = keepTime.current
      } catch {
        /* ignore */
      }
      setTime(keepTime.current)
      keepTime.current = null
    }
    if (autoPlay.current) {
      autoPlay.current = false
      void el.play().then(() => {
        setPhase('playing')
        startLoop()
      }).catch(() => {
        setPhase('paused')
      })
    }
  }, [song?.duration, startLoop])

  const restart = useCallback(() => {
    const a = audioRef.current
    if (!a) return
    setSoundActive(true)
    framesRef.current = []
    dotsRef.current = []
    smoothRef.current = []
    vadRef.current = createVad()
    setCatMode('silent')
    // к рестарту blob наверняка готов — подбираем отложенное, старт из памяти
    const p = pendingBlob.current
    if (p.minus && a.getAttribute('src') !== p.minus) a.src = p.minus
    const o = origRef.current
    if (o && p.orig && o.getAttribute('src') !== p.orig) o.src = p.orig
    pendingBlob.current = {}
    a.currentTime = 0
    useOrig.current = false
    a.muted = false
    if (o) {
      try {
        o.currentTime = 0
      } catch {
        /* ignore */
      }
      o.muted = true
      void o.play().catch(() => undefined)
    }
    setTime(0)
    setScore(null)
    setScored(false)
    void a.play().then(() => {
      setPhase('playing')
      startLoop()
    }).catch(() => {
      setPhase('paused')
    })
  }, [startLoop, setSoundActive])

  const backToCatalog = useCallback(() => {
    window.clearInterval(countTimer.current)
    audioRef.current?.pause()
    origRef.current?.pause()
    stopMic()
    setSoundActive(false)
    back()
  }, [back, stopMic, setSoundActive])

  const onEnded = useCallback(() => {
    cancelAnimationFrame(rafRef.current)
    setPhase('finished')
    setSoundActive(false)
    stopMic()
    origRef.current?.pause()
    if (song && framesRef.current.length > 10) {
      // нотный движок: ноты из квантованного эталона + кадры микрофона.
      // Пропуски «не поём» вычитаем: там слышно оригинал, очки не судим.
      const skips = song.skips ?? []
      const { notes, frames } = applySkips(extractNotes(song.pitch), framesRef.current, skips)
      const scores = scoreNotes(notes, frames)
      if (scores.some((s) => s.err !== null)) {
        const result = buildSongScore(scores)
        result.timing = scoreTiming(notes, frames)
        result.words = scoreWords(
          song.segments.flatMap((s) => s.words).filter((w) => !inSkip(skips, (w.s + w.e) / 2)),
          song.pitch,
          frames,
        )
        setScore(result)
        setScored(true)
      }
    }
  }, [stopMic, song, setSoundActive])

  const seek = useCallback((v: number) => {
    const a = audioRef.current
    if (!a) return
    // перемотка — удобный бесшовный момент добрать blob: рвём стрим один раз,
    // дальше всё из памяти
    const p = pendingBlob.current
    if (p.minus && a.getAttribute('src') !== p.minus) a.src = p.minus
    const o = origRef.current
    if (o && p.orig && o.getAttribute('src') !== p.orig) o.src = p.orig
    if (p.minus || p.orig) pendingBlob.current = {}
    a.currentTime = v
    if (o) {
      try {
        o.currentTime = v
      } catch {
        /* ignore */
      }
    }
    setTime(v)
    smoothRef.current = []
    dotsRef.current = dotsRef.current.filter((d) => d.t <= v + 0.2)
    framesRef.current = framesRef.current.filter((f) => f.t <= v + 0.2)
  }, [])

  const applyEdited = useCallback((segments: Segment[], editedSkips: SkipRange[]) => {
    if (!song) return
    openSong({ ...song, segments, skips: editedSkips })
  }, [song, openSong])

  const resetToBundle = useCallback(async () => {
    if (!song) return
    clearLocalLyrics(song.id)
    clearLocalSkips(song.id)
    try {
      openSong(await loadSong(song))
      setEditorReset((k) => k + 1)
    } catch (e) {
      console.error(e)
    }
  }, [song, openSong])

  // пробуем заранее показать список микрофонов
  useEffect(() => {
    navigator.mediaDevices?.enumerateDevices?.()
      .then((ds) => setDevices(ds.filter((d) => d.kind === 'audioinput')))
      .catch(() => undefined)
  }, [])

  // Предзагрузка аудио в blob: греем с открытия песни, подменяем src как только
  // догрелось. Нюанс стабильности: подмена src посреди пения СЛЫШИМОЙ дорожки
  // рвёт звук (перезагрузка декодера + AbortError стартующего play) — именно это
  // и давало «прерывисто» при переходе на плюс, когда blob оригинала приходил
  // уже по ходу песни. Поэтому слышимую играющую дорожку не трогаем: URL
  // откладываем в pendingBlob и подбираем на паузе/seek/рестарте. Фоновую
  // (приглушённую) можно менять свободно — её не слышно.
  useEffect(() => {
    if (!song) return
    const id = song.id
    pendingBlob.current = {}
    lastSyncRef.current = 0
    let cancelled = false
    const swapToBlob = (el: HTMLAudioElement | null, url: string, role: 'minus' | 'orig'): void => {
      if (cancelled || !el || el.getAttribute('src') === url) return
      const audible = role === 'minus' ? !useOrig.current : useOrig.current
      const playing = phaseRef.current === 'playing' || phaseRef.current === 'countdown'
      if (audible && playing) {
        pendingBlob.current[role === 'minus' ? 'minus' : 'orig'] = url
        return
      }
      const t = Number.isFinite(el.currentTime) ? el.currentTime : 0
      const ph = phaseRef.current
      el.src = url
      try {
        el.currentTime = t
      } catch {
        /* ignore */
      }
      // Подмена могла оборвать стартующий play() (AbortError): если поём
      // или стартуем — дожимаем play, иначе begin/пользователь сделают сами.
      if (ph === 'countdown' || ph === 'playing') {
        void el.play().catch(() => undefined)
      }
    }
    void warmSongAudio(id, 'minus', song.audio).then((url) => {
      if (url) swapToBlob(audioRef.current, url, 'minus')
    })
    if (song.original) {
      void warmSongAudio(id, 'orig', song.original).then((url) => {
        if (url) swapToBlob(origRef.current, url, 'orig')
      })
    }
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [song?.id])

  if (!song) return null
  const segs = segsSorted
  // окно: крупная + 2 следующие; спетость первой строки — прямо по часам, без гейтов
  const visible = frame.lines
  const sungNow = frame.sung
  const curLine = visible[0] ?? null

  return (
    <div className={`flex w-full flex-col px-5 max-sm:pb-[env(safe-area-inset-bottom)] ${singing ? 'h-full' : 'h-[calc(100%-64px)]'}`}>
      <audio ref={audioRef} src={song.audio}
        preload="auto" onEnded={onEnded}
        onWaiting={() => setWaiting(true)}
        onPlaying={() => setWaiting(false)}
        onCanPlay={() => setWaiting(false)}
        onLoadedMetadata={(e) => onBackingLoaded(e.currentTarget)} />
      {/* вторая дорожка: оригинал с вокалом для пропусков «не поём» */}
      {song.original && (
        <audio ref={origRef} src={song.original} preload="auto" muted
          onWaiting={() => { if (useOrig.current) setWaiting(true) }}
          onPlaying={() => setWaiting(false)}
          onCanPlay={() => setWaiting(false)} />
      )}

      {/* верхняя панель */}
      <TopBar
        title={song.title}
        timeSec={Math.floor(time)}
        duration={duration}
        withMic={withMic}
        micLive={withMic && (phase === 'playing' || phase === 'paused')}
        level={level}
        onBack={backToCatalog}
      />

      {/* текст как в настоящем караоке: текущая строка крупно + 2 следующие */}
      <div className="relative flex min-h-0 flex-1 flex-col justify-center gap-4 overflow-hidden rounded-2xl border border-border bg-surface-hover px-6 py-6 text-center sm:gap-6 sm:px-10">
        {segs.length === 0 ? (
          <p className="text-center text-sm text-muted">Текст не распознан — пойте под минус</p>
        ) : (
          <>
              {curLine && (
                <CurrentLine
                  s={curLine.s}
                  idx={curLine.i}
                  segs={segs}
                  sungNow={sungNow}
                  time={time}
                  ballOn={frame.ballOn}
                  centers={centers}
                  lineRef={lineRef}
                  wordRefs={wordRefs}
                  skips={song.skips ?? []}
                />
              )}
            <NextLines lines={nextLines} />
          </>
        )}
        {/* кот-индикатор микрофона: правый нижний угол блока текста */}
        <SingingCat mode={micLive ? catMode : 'silent'} />
      </div>

      {/* полоса тона: показываем гладкий контур, очки — по строгому */}
      {(song.pitchSmooth ?? song.pitch) && (phase === 'playing' || phase === 'paused') && (
        <div className="mt-2 rounded-2xl border border-border bg-surface-hover px-3 pb-1 pt-1.5 sm:mt-3 sm:px-4 sm:pt-2">
          <div className="hidden px-1 text-[11px] text-muted/70 sm:block">Попадание в ноты · золото — точно, светлые — почти, розовые — мимо</div>
          <PitchStrip pitch={(song.pitchSmooth ?? song.pitch)!} duration={duration || song.duration} time={Math.floor(time * 10) / 10} dotsRef={dotsRef} skips={song.skips} />
        </div>
      )}

      {/* прогресс: во время драга ползунок слушается пальца, а не звука */}
      <input type="range" min={0} max={Math.max(1, duration)} step={0.1}
        value={scrub ?? Math.min(time, duration || 0)}
        onChange={(e) => {
          const v = Number(e.target.value)
          setScrub(v)
          seek(v)
        }}
        onPointerUp={() => setScrub(null)}
        onPointerCancel={() => setScrub(null)}
        onKeyUp={() => setScrub(null)}
        onBlur={() => setScrub(null)}
        className="mt-3 w-full accent-primary" aria-label="Позиция в песне" />
      {waiting && (phase === 'playing' || phase === 'paused') && (
        <p className="mt-1 text-center text-xs text-muted tabular-nums">Буферизация…</p>
      )}

      {/* управление */}
      <ControlsBar
        phase={phase}
        volume={volume}
        onVolume={setVolume}
        backing={backing}
        hasOriginal={!!song.original}
        onToggle={togglePlay}
        onRestart={restart}
        onBacking={switchBacking}
      />

      {micError && <p className="pb-3 text-center text-[13px] text-danger/90">{micError}</p>}

      {/* стартовый экран */}
      <AnimatePresence>
        {phase === 'ready' && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
            className="fixed inset-0 z-20 grid place-items-center bg-black/70 p-5 backdrop-blur-sm">
            <motion.div initial={{ scale: 0.96, y: 10 }} animate={{ scale: 1, y: 0 }}
              className="w-full max-w-md rounded-3xl border border-border bg-surface p-7 text-center">
              <h2 className="text-xl font-semibold text-text">{song.title}</h2>
              <p className="mt-1 text-sm text-muted">
                {segs.length} строк · {formatTime(duration || song.duration)} · минус готов
              </p>
              {devices.length > 0 && (
                <select value={deviceId} onChange={(e) => setDeviceId(e.target.value)}
                  className="mt-5 w-full rounded-xl border border-border bg-surface-hover px-3 py-2.5 text-sm text-text outline-none">
                  <option value="">Микрофон по умолчанию</option>
                  {devices.map((d, i) => (
                    <option key={d.deviceId || i} value={d.deviceId}>
                      {d.label || `Микрофон ${i + 1}`}
                    </option>
                  ))}
                </select>
              )}
              <div className="mt-5 flex flex-col gap-2">
                <button onClick={() => void begin(true)}
                  className="flex items-center justify-center gap-2 rounded-2xl bg-primary py-3.5 text-[15px] font-semibold text-white transition hover:bg-primary-hover">
                  <Mic className="h-4.5 w-4.5" /> Петь с оценкой
                </button>
                <button onClick={() => void begin(false)}
                  className="rounded-2xl bg-surface-hover py-3.5 text-[15px] font-medium text-text transition hover:bg-border">
                  Просто подпевать
                </button>
              </div>
              <button onClick={backToCatalog} className="mt-4 text-[13px] text-muted hover:text-text">
                Назад к каталогу
              </button>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* отсчёт */}
      <AnimatePresence>
        {phase === 'countdown' && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
            className="fixed inset-0 z-20 grid place-items-center bg-black/70 backdrop-blur-sm">
            <motion.div key={count} initial={{ scale: 0.7, opacity: 0 }} animate={{ scale: 1, opacity: 1 }}
              className="text-[110px] font-bold text-primary tabular-nums">
              {count}
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* редактор текста */}
      <AnimatePresence>
        {editorOpen && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
            <Editor
              key={song.id}
              song={song}
              onClose={() => setEditorOpen(false)}
              onSave={applyEdited}
              onReset={() => void resetToBundle()}
              onSeek={seek}
              resetSignal={editorReset}
            />
          </motion.div>
        )}
      </AnimatePresence>

      {/* оценка */}
      <AnimatePresence>
        {phase === 'finished' && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
            className="fixed inset-0 z-20 grid place-items-center bg-black/70 p-5 backdrop-blur-sm">
            <motion.div initial={{ scale: 0.95, y: 12 }} animate={{ scale: 1, y: 0 }}
              className="w-full max-w-md rounded-3xl border border-border bg-surface p-8 text-center">
              {scored && score ? (
                <>
                  <motion.div initial={{ scale: 0.6 }} animate={{ scale: 1 }} transition={{ type: 'spring', stiffness: 200, damping: 14 }}
                    className="text-[84px] font-bold leading-none text-primary tabular-nums">
                    {score.score}
                  </motion.div>
                  <div className="mt-2 text-lg font-medium text-text">{gradeText(score.score)}</div>
                  <div className="mt-1 text-[13px] text-muted">
                    попаданий {score.hits}/{score.total} · точно {score.perfect} · ошибка {score.medianError} полутона
                  </div>
                  {score.timing && score.timing.sung > 0 && (
                    <div className="mt-1 text-[13px] text-muted">
                      ритм {score.timing.score} · вступление ±{score.timing.medianMs} мс
                    </div>
                  )}
                  {score.words && score.words.total > 0 && (
                    <div className="mt-1 text-[13px] text-muted">
                      слова {score.words.hit}/{score.words.total}
                    </div>
                  )}
                </>
              ) : (
                <>
                  <div className="flex justify-center">
                    {score === null && <Loader2 className="h-8 w-8 animate-spin text-muted" />}
                  </div>
                  <div className="mt-3 text-lg font-medium text-text">Готово!</div>
                  <div className="mt-1 text-[13px] text-muted">
                    {withMic ? 'Мало данных с микрофона для оценки — попробуйте ещё раз' : 'Вы пели без микрофона — так тоже отлично'}
                  </div>
                </>
              )}
              <div className="mt-6 flex gap-2">
                <button onClick={restart}
                  className="flex-1 rounded-2xl bg-primary py-3 text-[15px] font-semibold text-white transition hover:bg-primary-hover">
                  Ещё раз
                </button>
                <button onClick={backToCatalog}
                  className="flex-1 rounded-2xl bg-surface-hover py-3 text-[15px] font-medium text-text transition hover:bg-border">
                  Каталог
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
