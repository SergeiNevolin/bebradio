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

export interface PlayerProps {
  track: Track | null
  isPlaying: boolean
  position: number
  onPlayback: (action: string, extra?: Record<string, unknown>) => void
  likes: number
  dislikes: number
  userVote: 1 | -1 | 0
  onVote: (trackId: string, vote: 1 | -1 | 0) => void
  onSkipVote: () => void
  skipVoters: string[]
  currentUserId: string
}

export interface QueueProps {
  queue: Track[]
  currentIndex: number
}

export interface AddTrackProps {
  onAdd: (url: string) => Promise<{ success: boolean; error?: string }>
}
