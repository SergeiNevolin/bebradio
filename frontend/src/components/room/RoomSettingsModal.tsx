import type { RoomState } from '../../types'

interface RoomSettingsModalProps {
  room: RoomState
  onUpdate: (settings: {
    allow_anonymous_add?: boolean
    is_private?: boolean
    auto_radio?: boolean
    password?: string
  }) => void
  onDelete: () => void
  onClose: () => void
  settingsPassword: string
  setSettingsPassword: (v: string) => void
  deleting: boolean
  deleteError: string
}

export default function RoomSettingsModal({
  room,
  onUpdate,
  onDelete,
  onClose,
  settingsPassword,
  setSettingsPassword,
  deleting,
  deleteError,
}: RoomSettingsModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Room Settings</h3>
          <button className="btn-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          <label className="settings-toggle">
            <input
              type="checkbox"
              checked={room?.allow_anonymous_add ?? true}
              onChange={(e) => onUpdate({ allow_anonymous_add: e.target.checked })}
            />
            <span className="toggle-slider"></span>
            <span className="toggle-label">Allow anonymous users to add tracks</span>
          </label>
          <label className="settings-toggle">
            <input
              type="checkbox"
              checked={room?.is_private ?? false}
              onChange={(e) => onUpdate({ is_private: e.target.checked })}
            />
            <span className="toggle-slider"></span>
            <span className="toggle-label">Private room (hidden from public list)</span>
          </label>
          <label className="settings-toggle">
            <input
              type="checkbox"
              checked={room?.auto_radio ?? false}
              onChange={(e) => onUpdate({ auto_radio: e.target.checked })}
            />
            <span className="toggle-slider"></span>
            <span className="toggle-label">Auto-radio (keep playing related tracks when the queue runs out)</span>
          </label>

          <div className="settings-password" style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}>
            <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
              Room password
            </div>
            <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
              {room?.has_password
                ? 'This room is password protected.'
                : 'Anyone with the code can join.'}
            </div>
            <input
              type="password"
              placeholder={room?.has_password ? 'New password' : 'Set a password'}
              value={settingsPassword}
              onChange={(e) => setSettingsPassword(e.target.value)}
              style={{ width: '100%' }}
            />
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              <button
                className="btn btn-sm"
                disabled={!settingsPassword.trim()}
                onClick={() => onUpdate({ password: settingsPassword.trim() })}
              >
                {room?.has_password ? 'Change password' : 'Set password'}
              </button>
              {room?.has_password && (
                <button
                  className="btn btn-sm btn-secondary"
                  onClick={() => onUpdate({ password: '' })}
                >
                  Remove password
                </button>
              )}
            </div>
          </div>

          <div className="settings-danger" style={{ marginTop: 16, paddingTop: 16, borderTop: '1px solid var(--border, rgba(128,128,128,0.25))' }}>
            <div className="toggle-label" style={{ fontWeight: 600, marginBottom: 4 }}>
              Delete room
            </div>
            <div className="toggle-label" style={{ marginBottom: 8, opacity: 0.7, fontSize: 13 }}>
              Removes the room, its queue and chat for everyone. This cannot be undone.
            </div>
            {deleteError && <div className="error-msg" style={{ marginBottom: 8 }}>{deleteError}</div>}
            <button
              className="btn btn-sm btn-danger"
              onClick={onDelete}
              disabled={deleting}
            >
              {deleting ? 'Deleting...' : 'Delete this room'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
