import { useState, useEffect, useRef, useCallback } from 'react'
import type { NavigateFunction } from 'react-router-dom'
import { api } from '../lib/api'
import { getRoomAccess, setRoomAccess, clearRoomAccess } from '../lib/roomAccess'
import type { ChatMessage } from '../components/room/Chat'
import type { FloatingReaction } from '../components/room/Reactions'
import type { RoomState, StreamState } from '../types'

interface User {
  id: string
  username: string
}

export type SessionKind = 'room' | 'stream'
export type SessionState = RoomState | StreamState

function useSessionWebSocket(
  kind: SessionKind,
  sessionId: string | undefined,
  user: User | null,
  token: string | null,
  navigate: NavigateFunction,
) {
  const [room, setRoom] = useState<SessionState | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [locked, setLocked] = useState(false)
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [reactions, setReactions] = useState<FloatingReaction[]>([])
  const [userVote, setUserVote] = useState<1 | -1 | 0>(0)

  const prevTrackId = useRef<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const unmountedRef = useRef(false)

  const identity = useCallback(() => ({
    user_id: user?.id || '',
    username: user?.username || 'Anonymous',
  }), [user?.id, user?.username])

  const connectWs = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const params = new URLSearchParams()
    if (kind === 'room') {
      const access = getRoomAccess(sessionId!)
      if (access) params.set('access', access)
    }
    // Browsers can't set headers on WS upgrade: auth rides the query.
    if (token) params.set('token', token)
    const query = params.toString() ? `?${params.toString()}` : ''
    const path = kind === 'stream' ? `/ws/streams/${sessionId}` : `/ws/rooms/${sessionId}`
    const ws = new WebSocket(`${protocol}//${window.location.host}${path}${query}`)
    wsRef.current = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ action: 'hello', ...identity() }))
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.error && data.locked) {
          setLocked(true)
          ws.close()
          return
        }
        if (data.type === 'room_deleted' || data.type === 'stream_deleted') {
          unmountedRef.current = true
          ws.close()
          if (kind === 'room') clearRoomAccess(sessionId!)
          navigate(kind === 'stream' ? '/streams' : '/', { replace: true })
          return
        }
        if (data.type === 'reaction') {
          const item: FloatingReaction = {
            key: `${data.id}-${Math.random().toString(36).slice(2)}`,
            emoji: data.emoji,
            username: data.username,
            left: 8 + Math.random() * 78,
          }
          setReactions((prev) => [...prev, item])
          setTimeout(() => {
            setReactions((prev) => prev.filter((r) => r.key !== item.key))
          }, 3800)
          return
        }
        if (data.type === 'chat') {
          setChatMessages((prev) => [...prev.slice(-99), data.message])
        } else {
          setRoom(data as SessionState)
          if (data.messages) {
            setChatMessages(data.messages)
          }
        }
      } catch { /* ignore */ }
    }

    ws.onclose = () => {
      if (unmountedRef.current) return
      reconnectTimer.current = setTimeout(connectWs, 2000)
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [kind, sessionId, navigate, identity, token])

  const fetchRoom = useCallback(async (): Promise<SessionState | null> => {
    try {
      const data = (kind === 'stream'
        ? await api.getStream(sessionId!)
        : await api.getRoom(sessionId!)) as SessionState
      if (kind === 'room' && (data as RoomState).locked) {
        setRoom(data)
        setLocked(true)
        return null
      }
      if (kind === 'room' && (data as RoomState).access) {
        setRoomAccess(sessionId!, (data as RoomState).access!)
      }
      setLocked(false)
      setRoom(data)
      if (token && kind === 'room') {
        api.visitRoom(sessionId!)
      }
      return data
    } catch {
      setError(kind === 'stream' ? 'Поток не найден' : 'Room not found')
      return null
    } finally {
      setLoading(false)
    }
  }, [kind, sessionId, token])

  useEffect(() => {
    unmountedRef.current = false
    let cancelled = false
    fetchRoom().then((data) => {
      if (!cancelled && data) connectWs()
    })

    return () => {
      cancelled = true
      unmountedRef.current = true
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [sessionId, fetchRoom, connectWs])

  const sendWs = useCallback((msg: Record<string, unknown>) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg))
    }
  }, [])

  useEffect(() => {
    const trackId = room?.current_track?.id ?? null
    if (trackId !== prevTrackId.current) {
      prevTrackId.current = trackId
      setUserVote(0)
    }
  }, [room?.current_track?.id])

  const handleVote = useCallback((trackId: string, vote: 1 | -1 | 0) => {
    setUserVote(vote)
    sendWs({
      action: 'vote',
      user_id: user?.id || '',
      track_id: trackId,
      vote,
    })
  }, [sendWs, user?.id])

  const handleSkipVote = useCallback(() => {
    sendWs({
      action: 'skip_vote',
      user_id: user?.id || '',
    })
  }, [sendWs, user?.id])

  const handleReact = useCallback((emoji: string) => {
    sendWs({ action: 'reaction', emoji, ...identity() })
  }, [sendWs, identity])

  const handleSendChat = useCallback((text: string) => {
    sendWs({
      action: 'chat',
      text,
      user_id: user?.id || '',
      username: user?.username || 'Anonymous',
    })
  }, [sendWs, user?.id, user?.username])

  const handlePlayback = useCallback((action: string, extra: Record<string, unknown> = {}) => {
    sendWs({ action, ...extra })
  }, [sendWs])

  return {
    room,
    setRoom,
    loading,
    setLoading,
    error,
    setError,
    locked,
    setLocked,
    sendWs,
    fetchRoom,
    connectWs,
    chatMessages,
    setChatMessages,
    reactions,
    userVote,
    setUserVote,
    handleVote,
    handleSkipVote,
    handleReact,
    handleSendChat,
    handlePlayback,
  }
}

export function useRoomWebSocket(
  roomId: string | undefined,
  user: User | null,
  token: string | null,
  navigate: NavigateFunction,
) {
  return useSessionWebSocket('room', roomId, user, token, navigate)
}

export function useStreamWebSocket(
  streamId: string | undefined,
  user: User | null,
  token: string | null,
  navigate: NavigateFunction,
) {
  return useSessionWebSocket('stream', streamId, user, token, navigate)
}
