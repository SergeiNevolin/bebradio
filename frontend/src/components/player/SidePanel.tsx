import type { ReactNode } from 'react'
import styles from './SidePanel.module.css'

interface SidePanelProps {
  /** Accessible name (aside landmark). */
  label: string
  testId?: string
  /** Видна ли нижняя плашка плеера — под неё резервируем место внизу. */
  withPlayer?: boolean
  onClose?: () => void
  closeLabel?: string
  children: ReactNode
}

/**
 * Единая правая панель: очередь, превью караоке и всё будущее — один каркас,
 * меняется только наполнение. Встаёт рядом с контентом (см. PanelDock),
 * на мобильных — оверлей на всю ширину.
 */
export default function SidePanel({
  label,
  testId,
  withPlayer = false,
  onClose,
  closeLabel = 'Закрыть',
  children,
}: SidePanelProps) {
  return (
    <aside
      className={`${styles.panel} ${withPlayer ? styles.withPlayer : ''}`}
      aria-label={label}
      data-testid={testId}
    >
      {onClose && (
        <div className={styles.head}>
          <button type="button" className={styles.closeBtn} onClick={onClose} aria-label={closeLabel}>
            ×
          </button>
        </div>
      )}
      <div className={styles.scroll}>{children}</div>
    </aside>
  )
}
