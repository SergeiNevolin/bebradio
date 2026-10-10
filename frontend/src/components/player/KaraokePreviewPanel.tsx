import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { usePlayer, isOnKaraoke } from '../../context/PlayerContext'
import { useAuth } from '../../context/AuthContext'
import { useToast } from '../../context/ToastContext'
import { useKaraoke } from '../../karaoke/store'
import { loadSong, saveSongMeta } from '../../karaoke/lib/songs'
import { deleteSong } from '../../karaoke/lib/api'
import { karaokePreviewTrack } from '../../lib/karaokePreview'
import { tintForId } from '../../lib/mashupArt'
import EditKaraokeModal from '../karaoke/EditKaraokeModal'
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
  const { user } = useAuth()
  const [metaOpen, setMetaOpen] = useState(false)
  // Мета живьём из каталога — правки в модалке видны без переоткрытия.
  const liveMeta = useKaraoke((s) => s.songs.find((x) => x.id === previewSong?.id) ?? null)

  if (!previewSong) return null

  const track = karaokePreviewTrack(previewSong)
  const active = player.current?.id === track.id
  const playing = active && player.isPlaying
  // Править/удалять может загрузивший; легаси без владельца — любой вошедший.
  const canEdit =
    !!user && (previewSong.owner_id == null || previewSong.owner_id === user.id)

  const handlePreview = () => {
    toggleTrack(track, { blockedMessage: BLOCKED_MESSAGE })
  }

  const handleDelete = async () => {
    const songId = previewSong.id
    if (!window.confirm(`Удалить «${previewSong.title}» без возврата?`)) return
    try {
      await deleteSong(songId)
    } catch (e) {
      console.error(e)
      showToast('Не удалось удалить — попробуйте снова', 'error')
      return
    }
    const st = useKaraoke.getState()
    st.removeSong(songId)
    setPreviewSong(null)
    showToast('Песня удалена', 'success')
  }

  const handleEdit = () => {
    const st = useKaraoke.getState()
    st.requestEdit(previewSong.id)
    setPreviewSong(null)
    if (!isOnKaraoke(location.pathname)) navigate('/karaoke')
  }

  const handleSaveMeta = async (title: string, artist: string) => {
    const songId = previewSong.id
    await saveSongMeta(songId, { title, artist: artist || null })
    useKaraoke.getState().updateSongMeta(songId, title, artist || null)
    setPreviewSong({ ...previewSong, title })
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
    <>
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
          {previewSong.owner_name && (
            <div className={styles.uploader}>Загрузил {previewSong.owner_name}</div>
          )}
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
            {canEdit && (
              <>
                <button type="button" className="btn btn-secondary" onClick={() => setMetaOpen(true)}>
                  Изменить
                </button>
                <button type="button" className="btn btn-secondary" onClick={handleEdit}>
                  Редактировать караоке
                </button>
                <button type="button" className="btn btn-secondary" onClick={handleDelete}>
                  Удалить
                </button>
              </>
            )}
          </div>
        </div>
      </SidePanel>
      {metaOpen && canEdit && (
        <EditKaraokeModal
          song={liveMeta ?? {
            id: previewSong.id,
            title: previewSong.title,
            audio: previewSong.audio ?? '',
            lines: 0,
            duration: previewSong.duration ?? 0,
            language: previewSong.language ?? undefined,
          }}
          onSaveMeta={handleSaveMeta}
          onClose={() => setMetaOpen(false)}
        />
      )}
    </>
  )
}
