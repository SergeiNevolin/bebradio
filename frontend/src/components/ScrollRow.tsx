import { useRef, useState, useEffect, type ReactNode, type WheelEvent } from 'react'

interface ScrollRowProps {
  children: ReactNode
  className?: string
}

export default function ScrollRow({ children, className = '' }: ScrollRowProps) {
  const ref = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)

  const update = () => {
    const el = ref.current
    if (!el) return
    setCanLeft(el.scrollLeft > 4)
    setCanRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 4)
  }

  useEffect(() => {
    update()
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(update)
    ro.observe(el)
    el.addEventListener('scroll', update, { passive: true })
    return () => {
      ro.disconnect()
      el.removeEventListener('scroll', update)
    }
  }, [children])

  const scroll = (dir: -1 | 1) => {
    const el = ref.current
    if (!el) return
    el.scrollBy({ left: dir * el.clientWidth * 0.7, behavior: 'smooth' })
  }

  const onWheel = (e: WheelEvent) => {
    const el = ref.current
    if (!el) return
    if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
      e.preventDefault()
      el.scrollLeft += e.deltaY
    }
  }

  return (
    <div className={`scroll-row ${className}`}>
      {canLeft && (
        <button className="scroll-row-btn scroll-row-btn-left" onClick={() => scroll(-1)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6"/></svg>
        </button>
      )}
      <div
        className="scroll-row-track"
        ref={ref}
        onWheel={onWheel}
      >
        {children}
      </div>
      {canRight && (
        <button className="scroll-row-btn scroll-row-btn-right" onClick={() => scroll(1)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 18l6-6-6-6"/></svg>
        </button>
      )}
    </div>
  )
}
