import { useEffect } from 'react'
import KaraokeApp from '../karaoke/App'
import ErrorBoundary from '../karaoke/components/ErrorBoundary'
import { applyAccent, getStoredAccent } from '../karaoke/lib/theme'
import { usePlayer } from '../context/PlayerContext'
import '../karaoke/index.css'
import styles from './KaraokePage.module.css'

/**
 * Караоке — нативно внутри SPA bebradio (раньше было отдельным приложением
 * в iframe). Роут /karaoke, все запросы к karaoke-service — через единый
 * префикс /api/karaoke (см. lib/api.ts и nginx).
 * У караоке свой звук: общий плеер здесь прячется, а его playback ставим
 * на паузу при входе — как в комнатах.
 */
export default function KaraokePage() {
  const { player } = usePlayer()
  const pauseSharedPlayer = player.pause
  useEffect(() => {
    pauseSharedPlayer()
  }, [pauseSharedPlayer])

  // акцент из настроек bebradio → в токены караоке (--color-primary)
  useEffect(() => {
    applyAccent(getStoredAccent())
  }, [])

  return (
    <div className={styles.wrap}>
      <div className={`karaoke-root ${styles.root}`}>
        <ErrorBoundary>
          <KaraokeApp />
        </ErrorBoundary>
      </div>
    </div>
  )
}
