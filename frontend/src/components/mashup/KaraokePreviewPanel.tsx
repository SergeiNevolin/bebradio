import { useNavigate } from 'react-router-dom'
import { usePlayer } from '../../context/PlayerContext'
import { karaokePreviewTrack } from '../../lib/karaokePreview'
import { tintForId } from '../../lib/mashupArt'
import { MicIcon, PauseIcon, PlayIcon } from '../player/icons'
import styles from './KaraokePreviewPanel.module.css'

const BLOCKED_MESSAGE = 'В комнате играет эфир — послушать можно вне комнат'

function formatDuration(sec?: number | null): string {
  if (!sec || sec <= 0) return ''
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

/**
 * Панель превью караоке (справа): послушать перед тем как петь + кнопка «Спеть».
 * Открывается кликом по карточке на главной, автоплея нет.
 */
export default function KaraokePreviewPanel() {
  const { previewSong, setPreviewSong, player, toggleTrack } = usePlayer()
  const navigate = useNavigate()

  if (!previewSong) return null

  const track = karaokePreviewTrack(previewSong)
  const active = player.current?.id === track.id
  const playing = active && player.isPlaying

  const handlePreview = () => {
    toggleTrack(track, { blockedMessage: BLOCKED_MESSAGE })
  }

  const handleSing = () => {
    player.pause()
    setPreviewSong(null)
    navigate('/karaoke')
  }

  return (
    <aside
      className={`${styles.panel} ${player.current ? styles.withPlayer : ''}`}
      aria-label="Превью караоке"
      data-testid="karaoke-preview-panel"
    >
      <div className={styles.head}>
        <button
          type="button"
          className={styles.closeBtn}
          onClick={() => setPreviewSong(null)}
          aria-label="Закрыть превью"
        >
          ×
        </button>
      </div>

      <div className={styles.scroll}>
        <div className={styles.bodyPad}>
          <div className={styles.bigArt} style={{ background: tintForId(previewSong.id) }}>
            <MicIcon size={56} />
          </div>
          <div className={styles.bigTitle} title={previewSong.title}>
            {previewSong.title}
          </div>
          <div className={styles.bigMeta}>
            {[previewSong.language?.toUpperCase(), formatDuration(previewSong.duration)]
              .filter(Boolean)
              .join(' • ') || 'Караоке'}
          </div>
          <p className={styles.hint}>
            Превью — запись с вокалом. Минус для пения ждёт в караоке.
          </p>

          <div className={styles.actions}>
            <button type="button" className="btn btn-secondary" onClick={handlePreview}>
              {playing ? <PauseIcon size={16} /> : <PlayIcon size={16} />}
              {playing ? 'Пауза' : 'Слушать превью'}
            </button>
            <button type="button" className="btn" onClick={handleSing}>
              <MicIcon size={16} />
              Спеть
            </button>
          </div>
        </div>
      </div>
    </aside>
  )
}
