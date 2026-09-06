import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import ThemeToggle from '../components/ThemeToggle'

describe('ThemeToggle', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query === '(prefers-color-scheme: dark)' ? false : false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    })
  })

  it('renders sun icon when dark', () => {
    localStorage.setItem('theme', 'dark')
    render(<ThemeToggle />)
    expect(screen.getByText('☀️')).toBeInTheDocument()
  })

  it('renders moon icon when light', () => {
    localStorage.setItem('theme', 'light')
    render(<ThemeToggle />)
    expect(screen.getByText('🌙')).toBeInTheDocument()
  })

  it('toggles theme on click', () => {
    localStorage.setItem('theme', 'light')
    render(<ThemeToggle />)
    fireEvent.click(screen.getByText('🌙'))
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('sets data-theme on mount', () => {
    localStorage.setItem('theme', 'dark')
    render(<ThemeToggle />)
    expect(screen.getByText('☀️')).toBeInTheDocument()
  })

  it('toggles title attribute', () => {
    localStorage.setItem('theme', 'light')
    render(<ThemeToggle />)
    expect(screen.getByTitle('Switch to dark')).toBeInTheDocument()
    fireEvent.click(screen.getByText('🌙'))
    expect(screen.getByTitle('Switch to light')).toBeInTheDocument()
  })
})
