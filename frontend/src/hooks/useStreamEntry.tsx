import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import CreateStreamModal from '../components/stream/CreateStreamModal'

export function useStreamEntry() {
  const navigate = useNavigate()
  const [showCreate, setShowCreate] = useState(false)
  const [error, setError] = useState('')

  const openStreamById = useCallback(async (streamId: string): Promise<boolean> => {
    const id = streamId.trim().toUpperCase()
    if (!id) return false
    setError('')
    try {
      await api.getStream(id)
      navigate(`/stream/${id}`)
      return true
    } catch {
      setError('Поток не найден')
      return false
    }
  }, [navigate])

  const entryModals = showCreate
    ? <CreateStreamModal onClose={() => setShowCreate(false)} />
    : null

  return {
    error,
    openCreateModal: () => setShowCreate(true),
    openStreamById,
    entryModals,
  }
}
