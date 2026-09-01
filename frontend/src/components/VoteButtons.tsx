interface VoteButtonsProps {
  trackId: string
  likes: number
  dislikes: number
  userVote: number
  skipVoters: string[]
  currentUserId: string
  onVote: (trackId: string, vote: number) => void
  onSkipVote: () => void
}

export default function VoteButtons({
  trackId,
  likes,
  dislikes,
  userVote,
  skipVoters,
  currentUserId,
  onVote,
  onSkipVote,
}: VoteButtonsProps) {
  const hasVoted = skipVoters.includes(currentUserId)

  return (
    <div className="vote-buttons">
      <div className="vote-like-group">
        <button
          className={`vote-btn ${userVote === 1 ? 'vote-btn-active' : ''}`}
          onClick={() => onVote(trackId, userVote === 1 ? 0 : 1)}
          title="Like"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M1 21h4V9H1v12zm22-11c0-1.1-.9-2-2-2h-6.31l.95-4.57.03-.32c0-.41-.17-.79-.44-1.06L14.17 1 7.59 7.59C7.22 7.95 7 8.45 7 9v10c0 1.1.9 2 2 2h9c.83 0 1.54-.5 1.84-1.22l3.02-7.05c.09-.23.14-.47.14-.73v-2z"/>
          </svg>
          <span>{likes}</span>
        </button>
        <button
          className={`vote-btn ${userVote === -1 ? 'vote-btn-active-down' : ''}`}
          onClick={() => onVote(trackId, userVote === -1 ? 0 : -1)}
          title="Dislike"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M15 3H6c-.83 0-1.54.5-1.84 1.22l-3.02 7.05c-.09.23-.14.47-.14.73v2c0 1.1.9 2 2 2h6.31l-.95 4.57-.03.32c0 .41.17.79.44 1.06L9.83 23l6.59-6.59c.36-.36.58-.86.58-1.41V5c0-1.1-.9-2-2-2zm4 0v12h4V3h-4z"/>
          </svg>
          <span>{dislikes}</span>
        </button>
      </div>
      <button
        className={`vote-btn vote-skip ${hasVoted ? 'vote-skip-active' : ''}`}
        onClick={onSkipVote}
        title="Vote to skip"
      >
        <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
          <path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/>
        </svg>
        <span>Skip ({skipVoters.length})</span>
      </button>
    </div>
  )
}
