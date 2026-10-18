import { Suspense, lazy, useEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'
import { usePlayer, isPlayerHiddenPath, isOnKaraoke } from '../../context/PlayerContext'
import { useKaraoke } from '../../karaoke/store'
import MashupPlayer from './MashupPlayer'

// Оверлеи открываются только по действию (превью/очередь/разворот/профиль) —
// ленивые чанки, чтобы не парсить их на первой загрузке.
const KaraokePreviewPanel = lazy(() => import('./KaraokePreviewPanel'))
const NowPlayingPanel = lazy(() => import('./NowPlayingPanel'))
const NowPlayingModal = lazy(() => import('./NowPlayingModal'))
const ProfileModal = lazy(() => import('../ProfileModal'))

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
  // У превью караоке нет библиотечного трека — лайкать нечего.
  const likeHandler =
    player.current?.source === 'karaoke'
      ? undefined
      : (m: Parameters<typeof toggleLike>[0]) => {
          void toggleLike(m)
        }
  // В комнате свой эфир и своя плашка; в караоке поёт минус. UI плеера
  // прячем флагом hidden при СТАБИЛЬНОЙ позиции в дереве: любая смена ветки
  // (голый <audio>, обёртка-div) размонтирует MashupPlayer — у нового
  // элемента нет ни src, ни слушателей хука (его эффекты не перестреляют
  // на том же треке), и возврат из комнаты/пения оставляет немую плашку
  // с врущим значком Pause. Паузу при входе берут на себя комната
  // и эффект пения выше.
  const hidden = isPlayerHiddenPath(location.pathname) || singingKaraoke
  return (
    <MashupPlayer
      player={player}
      onToggleLike={likeHandler}
      onExpand={openExpanded}
      queueOpen={queueOpen}
      onToggleQueue={() => setQueueOpen((v) => !v)}
      hidden={hidden}
    />
  )
}

/**
 * Global overlays: queue drawer, expanded Now Playing and owner profiles.
 * Mounted once in App — Expand and Queue work identically on all pages,
 * with no navigation to /mashups.
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
    <Suspense fallback={null}>
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
    </Suspense>
  )
}
