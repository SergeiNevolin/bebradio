import { useEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'
import { usePlayer, isPlayerHiddenPath, isOnKaraoke } from '../../context/PlayerContext'
import { useKaraoke } from '../../karaoke/store'
import MashupPlayer from './MashupPlayer'
import NowPlayingPanel from './NowPlayingPanel'
import NowPlayingModal from './NowPlayingModal'
import KaraokePreviewPanel from './KaraokePreviewPanel'
import ProfileModal from '../ProfileModal'

/**
 * Стыковка правой панели: пока видна очередь или превью, страница ужимается
 * на ширину панели (класс + --panel-w на .app-root), и панель встаёт рядом
 * с контентом, а не поверх него. На мобильных и там, где панель скрыта
 * (комнаты, пение), класс снимается.
 */
export function PanelDock() {
  const { previewSong, queueOpen } = usePlayer()
  const location = useLocation()
  const karaokeActive = useKaraoke((s) => s.soundActive)
  const hidden =
    isPlayerHiddenPath(location.pathname) || (isOnKaraoke(location.pathname) && karaokeActive)
  const docked = (previewSong != null || queueOpen) && !hidden
  useEffect(() => {
    document.querySelector('.app-root')?.classList.toggle('with-panel', docked)
  }, [docked])
  return null
}

/**
 * Bottom player bar. Mounted once in App so it (and its <audio>) survives
 * navigation — playback continues on every page, karaoke included. It hides
 * (still keeping <audio> mounted) only in rooms, or while a karaoke song is
 * actually singing.
 */
export function GlobalPlayerBar() {
  const { player, toggleLike, openExpanded, queueOpen, setQueueOpen } = usePlayer()
  const location = useLocation()
  const karaokeActive = useKaraoke((s) => s.soundActive)

  // Как только караоке начинает петь — общий плеер сразу замолкает.
  const wasKaraokeActiveRef = useRef(karaokeActive)
  useEffect(() => {
    if (karaokeActive && !wasKaraokeActiveRef.current) player.pause()
    wasKaraokeActiveRef.current = karaokeActive
  }, [karaokeActive, player.pause])

  const singingKaraoke = isOnKaraoke(location.pathname) && karaokeActive
  if (isPlayerHiddenPath(location.pathname) || singingKaraoke) {
    // В комнате свой эфир и своя плашка; в караоке поёт минус. UI плеера
    // прячем, но <audio> держим смонтированным, чтобы не рвать состояние.
    return <audio ref={player.audioRef} preload="auto" />
  }
  // У превью караоке нет библиотечного трека — лайкать нечего.
  const likeHandler =
    player.current?.source === 'karaoke'
      ? undefined
      : (m: Parameters<typeof toggleLike>[0]) => {
          void toggleLike(m)
        }
  return (
    <MashupPlayer
      player={player}
      onToggleLike={likeHandler}
      onExpand={openExpanded}
      queueOpen={queueOpen}
      onToggleQueue={() => setQueueOpen((v) => !v)}
    />
  )
}

/**
 * Global overlays: queue drawer, expanded Now Playing and owner profiles.
 * Mounted once in App — Expand and Queue work identically on all pages,
 * with no navigation to /mashup.
 */
export function GlobalPlayerOverlays() {
  const {
    player,
    toggleTrack,
    toggleLike,
    queueOpen,
    setQueueOpen,
    expanded,
    setExpanded,
    profileUserId,
    setProfileUserId,
    previewSong,
    setPreviewSong,
  } = usePlayer()

  const queue =
    player.index >= 0 ? player.list.slice(player.index + 1, player.index + 4) : []

  // Протухшее превью: как только ПОСЛЕ его открытия заиграл обычный трек
  // (не превью) — закрываем, чтобы панель не показывала прошлое. Уже игравший
  // до открытия трек, пауза и само превью её не трогают.
  const baselineRef = useRef<string | null | undefined>(undefined)
  useEffect(() => {
    if (!previewSong) {
      baselineRef.current = undefined
      return
    }
    const id = player.current?.id ?? null
    if (baselineRef.current === undefined) {
      baselineRef.current = id
      return
    }
    if (id && id !== baselineRef.current && player.current?.source !== 'karaoke') {
      setPreviewSong(null)
    }
  })

  const location = useLocation()
  const karaokeActive = useKaraoke((s) => s.soundActive)
  const playerHidden =
    isPlayerHiddenPath(location.pathname) || (isOnKaraoke(location.pathname) && karaokeActive)

  const handleToggleLike =
    player.current?.source === 'karaoke'
      ? undefined
      : (m: Parameters<typeof toggleLike>[0]) => {
          void toggleLike(m)
        }

  return (
    <>
      {previewSong && !playerHidden && <KaraokePreviewPanel />}

      {queueOpen && !playerHidden && !previewSong && (
        <NowPlayingPanel
          current={player.current}
          queue={queue}
          loading={false}
          onPlayFromQueue={toggleTrack}
          onToggleLike={handleToggleLike}
          onOpenProfile={setProfileUserId}
          onClose={() => setQueueOpen(false)}
          playerVisible={player.current != null}
        />
      )}

      {expanded && player.current && !playerHidden && (
        <NowPlayingModal
          player={player}
          queue={queue}
          onToggleLike={handleToggleLike}
          onOpenProfile={setProfileUserId}
          onClose={() => setExpanded(false)}
        />
      )}

      {profileUserId && (
        <ProfileModal userId={profileUserId} onClose={() => setProfileUserId(null)} />
      )}
    </>
  )
}
