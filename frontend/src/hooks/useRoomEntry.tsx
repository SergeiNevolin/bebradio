import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import CreateRoomModal from '../components/room/CreateRoomModal'
import JoinRoomModal from '../components/room/JoinRoomModal'
import RoomPasswordModal from '../components/room/RoomPasswordModal'

/**
 * Shared room entry flow (create / join-by-code / password), used by both
 * the Home hero and the Rooms page so the logic lives in exactly one place.
 */
export function useRoomEntry() {
  const navigate = useNavigate()
  const [showCreate, setShowCreate] = useState(false)
  const [showJoin, setShowJoin] = useState(false)
  const [passwordPrompt, setPasswordPrompt] = useState<string | null>(null)
  const [error, setError] = useState('')

  const openRoomById = useCallback(async (roomId: string): Promise<boolean> => {
    const code = roomId.trim().toUpperCase()
    if (!code) return false
    setError('')
    try {
      const data = await api.getRoomByCode(code)
      if ((data as Record<string, unknown>).locked) {
        setPasswordPrompt(code)
        return false
      }
      navigate(`/room/${code}`)
      return true
    } catch {
      setError('Room not found')
      return false
    }
  }, [navigate])

  const entryModals = (
    <>
      {showCreate && <CreateRoomModal onClose={() => setShowCreate(false)} />}
      {showJoin && (
        <JoinRoomModal
          onClose={() => setShowJoin(false)}
          onLocked={(code) => {
            setShowJoin(false)
            setPasswordPrompt(code)
          }}
        />
      )}
      {passwordPrompt && (
        <RoomPasswordModal code={passwordPrompt} onClose={() => setPasswordPrompt(null)} />
      )}
    </>
  )

  return {
    error,
    openCreateModal: () => setShowCreate(true),
    openJoinModal: () => setShowJoin(true),
    openRoomById,
    entryModals,
  }
}
