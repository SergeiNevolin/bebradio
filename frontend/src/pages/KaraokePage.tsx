import KaraokeApp from '../karaoke/App'
import ErrorBoundary from '../karaoke/components/ErrorBoundary'
import { useKaraoke } from '../karaoke/store'
import { usePlayer } from '../context/PlayerContext'
import '../karaoke/index.css'
import styles from './KaraokePage.module.css'

/**
 * Караоке — нативно внутри SPA bebradio (раньше было отдельным приложением
 * в iframe). Роут /karaoke, все запросы к karaoke-service — через единый
 * префикс /api/karaoke (см. lib/api.ts и nginx).
 * Общий плеер работает и здесь: он прячется (и ставит паузу) только на время
 * пения конкретной песни — флаг soundActive в karaoke store. Полоса под плеер
 * (.barClear) резервируется только пока его плашка реально видна, т.е. есть
 * текущий трек и песня не поётся, — иначе внизу оставалась бы пустая дыра.
 * Тема и акцент общие: их задаёт App.tsx (--bg/--primary), токены караоке
 * ссылаются на те же переменные (см. karaoke/index.css).
 */
export default function KaraokePage() {
  const karaokeActive = useKaraoke((s) => s.soundActive)
  const { player } = usePlayer()
  const reservePlayer = !!player.current && !karaokeActive

  return (
    <div className={styles.wrap}>
      <div className={`karaoke-root ${styles.root} ${reservePlayer ? styles.barClear : ''}`}>
        <ErrorBoundary>
          <KaraokeApp />
        </ErrorBoundary>
      </div>
    </div>
  )
}
