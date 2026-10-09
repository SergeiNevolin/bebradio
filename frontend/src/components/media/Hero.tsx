import type { ReactNode } from 'react'
import styles from './Hero.module.css'

interface HeroProps {
  title: string
  sub?: ReactNode
  actions?: ReactNode
  /** Правая колонка статистики (hero комнат). */
  stats?: ReactNode
}

/**
 * Единый hero страниц: панель с заголовком, подписью, действиями
 * и опциональной статистикой. Одинаковые отступы везде: снизу 20px.
 */
export default function Hero({ title, sub, actions, stats }: HeroProps) {
  return (
    <div className={styles.hero}>
      <div className={styles.main}>
        <h1 className={styles.title}>{title}</h1>
        {sub && <p className={styles.sub}>{sub}</p>}
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
      {stats && <div className={styles.stats}>{stats}</div>}
    </div>
  )
}
