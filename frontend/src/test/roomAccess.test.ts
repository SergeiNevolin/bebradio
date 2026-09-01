import { describe, it, expect, beforeEach } from 'vitest'
import { getRoomAccess, setRoomAccess, clearRoomAccess } from '../lib/roomAccess'

describe('roomAccess', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns null when nothing is stored', () => {
    expect(getRoomAccess('ABC123')).toBeNull()
  })

  it('stores and reads a token, keyed case-insensitively by room id', () => {
    setRoomAccess('abc123', 'tok-1')
    expect(getRoomAccess('ABC123')).toBe('tok-1')
    expect(getRoomAccess('abc123')).toBe('tok-1')
  })

  it('clears a stored token', () => {
    setRoomAccess('ABC123', 'tok-1')
    clearRoomAccess('ABC123')
    expect(getRoomAccess('ABC123')).toBeNull()
  })
})
