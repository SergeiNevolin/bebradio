import { useLocation } from 'react-router-dom'
import { usePlayer, isPlayerHiddenPath } from '../../context/PlayerContext'
import MashupPlayer from '../mashup/MashupPlayer'
import NowPlayingPanel from '../mashup/NowPlayingPanel'
import NowPlayingModal from '../mashup/NowPlayingModal'
import KaraokePreviewPanel from '../mashup/KaraokePreviewPanel'
import ProfileModal from '../ProfileModal'

/**
 * Bottom player bar. Mounted once in App so it (and its <audio>) survives
 * navigation — playback continues on every page.
 */
export function GlobalPlayerBar() {
  const { player, toggleLike, openExpanded, queueOpen, setQueueOpen } = usePlayer()
  const location = useLocation()
  if (isPlayerHiddenPath(location.pathname)) {
    // В комнате свой эфир и своя плашка: UI плеера прячем, но <audio>
    // держим смонтированным, чтобы не рвать состояние хука.
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
  } = usePlayer()

  const queue =
    player.index >= 0 ? player.list.slice(player.index + 1, player.index + 4) : []

  const location = useLocation()
  const playerHidden = isPlayerHiddenPath(location.pathname)

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
