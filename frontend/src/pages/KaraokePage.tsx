import { useEffect } from 'react'
import { usePlayer } from '../context/PlayerContext'
import styles from './KaraokePage.module.css'

/**
 * Караоке — отдельное приложение (свой бэкенд/тема) под /karaoke/ в iframe.
 * Встраиваем под навбаром bebradio на всю ширину вьюпорта.
 * У караоке свой звук: общий плеер здесь прячется, а его playback ставим
 * на паузу при входе — как в комнатах.
 */
export default function KaraokePage() {
  const { player } = usePlayer()
  const pauseSharedPlayer = player.pause
  useEffect(() => {
    pauseSharedPlayer()
  }, [pauseSharedPlayer])

  return (
    <div className={styles.wrap}>
      <iframe src="/karaoke/" title="Караоке" allow="autoplay" className={styles.frame} />
    </div>
  )
}
