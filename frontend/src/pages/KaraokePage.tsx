import styles from './KaraokePage.module.css'

/**
 * Караоке — отдельное приложение (свой бэкенд/тема) под /karaoke/ в iframe.
 * Встраиваем под навбаром bebradio на всю ширину вьюпорта.
 */
export default function KaraokePage() {
  return (
    <div className={styles.wrap}>
      <iframe src="/karaoke/" title="Караоке" allow="autoplay" className={styles.frame} />
    </div>
  )
}
