import type { Track } from '../types'

interface QueueProps {
  queue: Track[]
  currentIndex: number
  /** Auto-radio is fetching related tracks right now. */
  searching?: boolean
}

export default function Queue({ queue, currentIndex, searching = false }: QueueProps) {
  if (!queue.length) {
    return (
      <div className="queue">
        <h3>Queue</h3>
        <div className="queue-empty">
          {searching ? (
            <span className="queue-searching">
              <span className="radio-spinner" aria-hidden="true" />
              Radio is finding tracks…
            </span>
          ) : (
            'No tracks yet. Add a YouTube link above.'
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="queue">
      <h3>Queue ({queue.length})</h3>
      <div className="queue-list">
        {queue.map((track, i) => (
          <div
            key={track.id}
            className={`queue-item ${i === currentIndex ? 'active' : ''}`}
          >
            <span className="queue-item-num">{i + 1}</span>
            {track.thumbnail && (
              <img className="queue-item-thumb" src={track.thumbnail} alt="" />
            )}
            <div className="queue-item-info">
              <div className="title">{track.title}</div>
              <div className="artist">{track.artist}</div>
            </div>
          </div>
        ))}
      </div>
      {searching && (
        <div className="queue-searching queue-searching-foot">
          <span className="radio-spinner" aria-hidden="true" />
          Radio is finding more tracks…
        </div>
      )}
    </div>
  )
}
