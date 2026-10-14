import { useEffect } from 'react'
import type { VoiceMode } from '../lib/voiceActivity'

export const CAT_LOUD_SRC = '/cat-loud-sing.png'
export const CAT_SOFT_SRC = '/cat-soft-sing.png'
export const CAT_SILENT_SRC = '/cat-silent.png'

export type CatMode = VoiceMode

interface SingingCatProps {
  /** режим кота: молчит / тихо поёт / громко поёт */
  mode: CatMode
}

const SRC: Record<CatMode, string> = {
  silent: CAT_SILENT_SRC,
  soft: CAT_SOFT_SRC,
  loud: CAT_LOUD_SRC,
}

const MODES: CatMode[] = ['silent', 'soft', 'loud']

/**
 * Кот-индикатор в правом нижнем углу блока текста.
 * Все три картинки всегда в DOM — активна ровно одна (остальные display:none),
 * смена жёсткая без кроссфейда: при наложении полупрозрачных кадров
 * просвечивает фон. Декоративный: pointer-events none, aria-hidden.
 */
export default function SingingCat({ mode }: SingingCatProps) {
  // Греем кадры заранее, чтобы переключение было мгновенным
  useEffect(() => {
    for (const src of [CAT_LOUD_SRC, CAT_SOFT_SRC, CAT_SILENT_SRC]) {
      const img = new Image()
      img.src = src
    }
  }, [])

  return (
    <div
      aria-hidden
      className="pointer-events-none absolute bottom-3 right-3 z-10 h-32 w-32 select-none sm:bottom-4 sm:right-4 sm:h-44 sm:w-44"
    >
      {MODES.map((m) => (
        <img
          key={m}
          src={SRC[m]}
          alt=""
          draggable={false}
          className={`absolute inset-0 h-full w-full object-contain ${
            mode === m ? '' : 'hidden'
          }`}
        />
      ))}
    </div>
  )
}
