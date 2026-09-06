interface RoomPasswordGateProps {
  roomName: string
  passwordInput: string
  setPasswordInput: (v: string) => void
  onUnlock: () => void
  unlocking: boolean
  error: string
  onBack: () => void
}

export default function RoomPasswordGate({
  roomName,
  passwordInput,
  setPasswordInput,
  onUnlock,
  unlocking,
  error,
  onBack,
}: RoomPasswordGateProps) {
  return (
    <div className="loading">
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, maxWidth: 360 }}>
        <h2>🔒 {roomName || 'Room'}</h2>
        <p style={{ fontSize: 14, textAlign: 'center' }}>This room is password protected.</p>
        <input
          type="password"
          autoFocus
          placeholder="Room password"
          value={passwordInput}
          onChange={(e) => setPasswordInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onUnlock()}
          style={{ width: '100%' }}
        />
        {error && <div className="error-msg">{error}</div>}
        <button className="btn" onClick={onUnlock} disabled={unlocking || !passwordInput} style={{ width: '100%' }}>
          {unlocking ? 'Checking...' : 'Enter room'}
        </button>
        <button className="btn btn-secondary" onClick={onBack}>
          Back to Home
        </button>
      </div>
    </div>
  )
}
