import { useRef, useState, useEffect, type MutableRefObject, type ReactNode, type WheelEvent } from 'react'
import styles from './ScrollRow.module.css'

interface ScrollRowProps {
  children: ReactNode
  className?: string
  /** Ref на скролл-бокс рейла (корень для IntersectionObserver снаружи). */
  trackRef?: MutableRefObject<HTMLDivElement | null> | ((el: HTMLDivElement | null) => void)
}

export default function ScrollRow({ children, className = '', trackRef }: ScrollRowProps) {
  const ref = useRef<HTMLDivElement | null>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)

  const setTrack = (el: HTMLDivElement | null) => {
    ref.current = el
    if (!trackRef) return
    if (typeof trackRef === 'function') trackRef(el)
    else trackRef.current = el
  }

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
    <div className={`${styles.scrollRow} ${className}`}>
      {canLeft && (
        <button className={`${styles.scrollRowBtn} ${styles.scrollRowBtnLeft}`} onClick={() => scroll(-1)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6"/></svg>
        </button>
      )}
      <div
        className={styles.scrollRowTrack}
        ref={setTrack}
        onWheel={onWheel}
      >
        {children}
      </div>
      {canRight && (
        <button className={`${styles.scrollRowBtn} ${styles.scrollRowBtnRight}`} onClick={() => scroll(1)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 18l6-6-6-6"/></svg>
        </button>
      )}
    </div>
  )
}
