export interface Track {
  id: string
  title: string
  artist: string
  url: string
  thumbnail: string
  duration: number
  added_by: string
  likes?: number
  dislikes?: number
}

export type MashupStatus = 'processing' | 'ready' | 'failed'

export interface Mashup {
  id: string
  owner_id: string
  owner_name?: string
  title: string
  artist: string
  duration: number
  size_bytes: number
  status: MashupStatus
  error?: string
  has_cover: boolean
  plays: number
  likes: number
  // Whether the current viewer has liked this mashup.
  liked?: boolean
  created_at: string
  // Present only once status === 'ready'.
  stream_url?: string
  // Present only when has_cover is true.
  cover_url?: string
}

export interface RoomListItem {
  id: string
  name: string
  user_count: number
  track_count?: number
  is_playing: boolean
  has_password: boolean
}

export interface RoomState {
  id: string
  name: string
  owner_id: string
  queue: Track[]
  current_index: number
  is_playing: boolean
  position: number
  current_track: Track | null
  user_count: number
  listeners?: { id: string; name: string }[]
  allow_anonymous_add: boolean
  is_private: boolean
  auto_radio?: boolean
  // True while auto-radio is fetching related tracks in the background.
  radio_searching?: boolean
  has_password: boolean
  track_votes: { likes: number; dislikes: number }
  skip_voters: string[]
  messages?: { id: string; user_id: string; username: string; text: string; created_at: number }[]
  // Present only on the stripped payload returned for a locked room the
  // caller has not unlocked yet.
  locked?: boolean
  // Issued to the room owner (and after a successful password check via /join).
  access?: string
}
