import { useEffect } from 'react'
import ProfileCard from './ProfileCard'

interface ProfileModalProps {
  userId: string
  onClose: () => void
}

// Shows a user's profile in an overlay so it can be opened from inside a room
// without navigating away and interrupting playback.
export default function ProfileModal({ userId, onClose }: ProfileModalProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Profile</h3>
          <button className="btn-close" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          <ProfileCard userId={userId} onNavigate={onClose} />
        </div>
      </div>
    </div>
  )
}
