import { useState, useEffect } from 'react'

export default function ThemeToggle() {
  const [dark, setDark] = useState(() => {
    const saved = localStorage.getItem('theme')
    if (saved) return saved === 'dark'
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light')
    localStorage.setItem('theme', dark ? 'dark' : 'light')
  }, [dark])

  return (
    <button
      className="theme-toggle"
      onClick={() => setDark((d) => !d)}
      title={dark ? 'Switch to light' : 'Switch to dark'}
    >
      {dark ? '☀️' : '🌙'}
    </button>
  )
}
