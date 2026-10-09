import { useRef, useState, useEffect, type MutableRefObject, type ReactNode } from 'react'
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

  // Вертикальное колесо крутит рейл, а не страницу. React вешает onWheel
  // как passive (preventDefault там молча игнорируется), поэтому слушатель
  // нативный с passive: false. Когда полке некуда ехать — страницу отдаём.
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const onWheelNative = (e: WheelEvent) => {
      if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return
      const canDown = el.scrollLeft + el.clientWidth < el.scrollWidth - 1
      const canUp = el.scrollLeft > 1
      if ((e.deltaY > 0 && !canDown) || (e.deltaY < 0 && !canUp)) return
      e.preventDefault()
      el.scrollLeft += e.deltaY
    }
    el.addEventListener('wheel', onWheelNative, { passive: false })
    return () => el.removeEventListener('wheel', onWheelNative)
  }, [])

  const scroll = (dir: -1 | 1) => {
    const el = ref.current
    if (!el) return
    el.scrollBy({ left: dir * el.clientWidth * 0.7, behavior: 'smooth' })
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
