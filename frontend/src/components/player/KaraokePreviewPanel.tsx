import { useLocation, useNavigate } from 'react-router-dom'
import { usePlayer, isOnKaraoke } from '../../context/PlayerContext'
import { useToast } from '../../context/ToastContext'
import { useKaraoke } from '../../karaoke/store'
import { loadSong } from '../../karaoke/lib/songs'
import { karaokePreviewTrack } from '../../lib/karaokePreview'
import { tintForId } from '../../lib/mashupArt'
import { MicIcon, PauseIcon, PlayIcon } from './icons'
import SidePanel from './SidePanel'
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
  const location = useLocation()
  const { showToast } = useToast()

  if (!previewSong) return null

  const track = karaokePreviewTrack(previewSong)
  const active = player.current?.id === track.id
  const playing = active && player.isPlaying

  const handlePreview = () => {
    toggleTrack(track, { blockedMessage: BLOCKED_MESSAGE })
  }

  const handleSing = async () => {
    const songId = previewSong.id
    player.pause()
    setPreviewSong(null)
    if (!isOnKaraoke(location.pathname)) {
      navigate('/karaoke')
      return
    }
    // Уже на странице караоке — открываем песню сразу, без навигации.
    const st = useKaraoke.getState()
    if (st.loadingSong) return
    const meta = st.songs.find((s) => s.id === songId)
    if (!meta) return
    st.setLoadingSong(true)
    try {
      const data = await loadSong(meta)
      const cur = useKaraoke.getState()
      cur.pushRecent(meta.id)
      cur.openSong(data)
    } catch (e) {
      console.error(e)
      showToast('Не открылась — проверьте файлы песни', 'error')
    } finally {
      useKaraoke.getState().setLoadingSong(false)
    }
  }

  return (
    <SidePanel
      label="Превью караоке"
      testId="karaoke-preview-panel"
      withPlayer={player.current != null}
      onClose={() => setPreviewSong(null)}
      closeLabel="Закрыть превью"
    >
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
      </SidePanel>
  )
}
