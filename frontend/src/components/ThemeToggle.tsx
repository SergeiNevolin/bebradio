import { useState } from 'react'
import { getStoredTheme, toggleTheme } from '../lib/theme'
import styles from './ThemeToggle.module.css'

export default function ThemeToggle() {
  const [dark, setDark] = useState(() => getStoredTheme() === 'dark')

  const handleToggle = () => {
    const next = toggleTheme()
    setDark(next === 'dark')
  }

  return (
    <button
      className={styles.themeToggle}
      onClick={handleToggle}
      title={dark ? 'Switch to light' : 'Switch to dark'}
    >
      {dark ? '☀️' : '🌙'}
    </button>
  )
}
