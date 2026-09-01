// Per-room access tokens for password-protected rooms. Stored per browser so a
// listener who has already entered the password does not have to retype it on
// every visit. Wrapped in try/catch because localStorage can throw in private
// browsing modes.

const key = (roomId: string) => `room_access_${roomId.toUpperCase()}`

export function getRoomAccess(roomId: string): string | null {
  try {
    return localStorage.getItem(key(roomId))
  } catch {
    return null
  }
}

export function setRoomAccess(roomId: string, token: string): void {
  try {
    localStorage.setItem(key(roomId), token)
  } catch {
    /* ignore */
  }
}

export function clearRoomAccess(roomId: string): void {
  try {
    localStorage.removeItem(key(roomId))
  } catch {
    /* ignore */
  }
}
