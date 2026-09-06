import type { RoomState } from '../../types'
import styles from '../../pages/Room.module.css'

interface RoomHeaderProps {
  room: RoomState
  roomId: string
  isOwner: boolean
  copied: boolean
  onCopyCode: () => void
  onShare: () => void
  onOpenSettings: () => void
}

export default function RoomHeader({
  room,
  roomId,
  isOwner,
  copied,
  onCopyCode,
  onShare,
  onOpenSettings,
}: RoomHeaderProps) {
  return (
    <header className={styles.roomHeader}>
      <div className={styles.roomHeaderLeft}>
        <div className={styles.roomTitleGroup}>
          <h1 className={styles.roomTitle}>
            {room?.has_password && <span title="Password protected">🔒 </span>}
            {room?.name || 'Room'}
          </h1>
          <div className={styles.roomMeta}>
            <span className={styles.roomStatus}>
              <span className={styles.statusDot} />
              {room?.user_count || 0} listening
            </span>
            {room?.auto_radio && (
              <>
                <span className={styles.roomDivider}>·</span>
                {room?.radio_searching ? (
                  <span className="radio-badge is-searching" title="Auto-radio is finding related tracks">
                    <span className="radio-spinner" aria-hidden="true" />
                    Finding tracks…
                  </span>
                ) : (
                  <span className="radio-badge" title="Auto-radio keeps the queue full">📻 Radio</span>
                )}
              </>
            )}
            <span className={styles.roomDivider}>·</span>
            <button
              className={styles.roomCodeBtn}
              onClick={onCopyCode}
              title={copied ? 'Copied!' : 'Click to copy room code'}
            >
              {copied ? '✓ Copied' : roomId}
            </button>
          </div>
        </div>
      </div>
      <div className={styles.roomHeaderRight}>
        <button className="btn btn-ghost btn-sm" onClick={onShare} title="Share room">
          Share
        </button>
        {isOwner && (
          <button className="btn btn-ghost btn-sm" onClick={onOpenSettings}>
            Settings
          </button>
        )}
      </div>
    </header>
  )
}
