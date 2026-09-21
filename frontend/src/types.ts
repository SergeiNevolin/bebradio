// Where a track came from. 'youtube' = resolved external URL, 'upload' =
// user mashup from the library. New providers (spotify, soundcloud, ...)
// extend this union; queue/room UI must fall back to the raw string for
// unknown sources instead of hiding them.
export type TrackSource = 'youtube' | 'upload' | (string & {})

export type TrackStatus = 'ready' | 'processing' | 'failed'

export interface Track {
  id: string
  source: TrackSource
  title: string
  artist: string
  // Playable audio URL. Empty while an upload is still processing.
  // YouTube queue entries use /api/music/<media_id>, uploads use
  // /api/tracks/<id>/audio.
  url: string
  // YouTube thumbnail or upload cover. Empty when there is none.
  thumbnail: string
  duration: number
  added_by: string
  owner_id: string
  owner_name?: string
  size_bytes: number
  status: TrackStatus
  error?: string
  has_cover: boolean
  plays: number
  likes: number
  // Whether the current viewer has liked this track.
  liked?: boolean
  created_at: string
  // Room queue entries only (votes, not library likes).
  dislikes?: number
}

export interface RoomListItem {
  id: string
  name: string
  user_count: number
  track_count?: number
  is_playing: boolean
  has_password: boolean
  // True for autodj rooms (auto_radio). Exposed by GET /api/rooms so the
  // home page can shelf them as radio stations.
  auto_radio?: boolean
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
