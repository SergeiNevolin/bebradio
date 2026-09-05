export interface Station {
  id: string
  name: string
  icon: string
  description: string
  seedQuery: string
}

export const stations: Station[] = [
  {
    id: 'chill',
    name: 'Chill',
    icon: '🍃',
    description: 'Relax & unwind',
    seedQuery: 'chill mix',
  },
  {
    id: 'party',
    name: 'Party',
    icon: '🎉',
    description: 'Party vibes',
    seedQuery: 'party mix',
  },
  {
    id: 'electronic',
    name: 'Electronic',
    icon: '🎧',
    description: 'EDM & dance',
    seedQuery: 'electronic music mix',
  },
  {
    id: 'rock',
    name: 'Rock',
    icon: '🎸',
    description: 'Rock anthems',
    seedQuery: 'rock mix',
  },
  {
    id: 'hiphop',
    name: 'Hip-Hop',
    icon: '🎤',
    description: 'Hip-hop beats',
    seedQuery: 'hip hop mix',
  },
  {
    id: 'indie',
    name: 'Indie',
    icon: '🎵',
    description: 'Indie gems',
    seedQuery: 'indie music mix',
  },
]
