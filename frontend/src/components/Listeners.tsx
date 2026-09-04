import { memo } from 'react'

interface Listener {
  id: string
  name: string
}

interface ListenersProps {
  listeners: Listener[]
  count: number
  max?: number
}

function Listeners({ listeners, count, max = 4 }: ListenersProps) {
  const shown = listeners.slice(0, max)
  const extra = count - shown.length

  return (
    <span
      className="listeners"
      title={listeners.length ? listeners.map((l) => l.name).join(', ') : undefined}
    >
      <span className="status-dot" />
      <span className="listeners-count">{count} listening</span>
      {shown.length > 0 && (
        <span className="listener-chips">
          {shown.map((l) => (
            <span key={l.id} className="listener-chip">{l.name}</span>
          ))}
          {extra > 0 && <span className="listener-chip listener-chip-more">+{extra}</span>}
        </span>
      )}
    </span>
  )
}

export default memo(Listeners)
