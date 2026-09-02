import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import AccentPicker from '../components/AccentPicker'

beforeEach(() => {
  localStorage.clear()
  document.documentElement.removeAttribute('style')
})

describe('AccentPicker', () => {
  it('opens the palette on click', () => {
    render(<AccentPicker />)
    expect(screen.queryByTitle('Blue')).not.toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Accent color'))
    expect(screen.getByTitle('Blue')).toBeInTheDocument()
  })

  it('applies a chosen preset to :root and persists it', () => {
    render(<AccentPicker />)
    fireEvent.click(screen.getByLabelText('Accent color'))
    fireEvent.click(screen.getByTitle('Violet'))
    expect(document.documentElement.style.getPropertyValue('--primary')).toBe('#7c3aed')
    expect(localStorage.getItem('accent')).toBe('#7c3aed')
  })

  it('clears the override when the default preset is chosen', () => {
    localStorage.setItem('accent', '#2563eb')
    render(<AccentPicker />)
    fireEvent.click(screen.getByLabelText('Accent color'))
    fireEvent.click(screen.getByTitle('Green'))
    expect(document.documentElement.style.getPropertyValue('--primary')).toBe('')
    expect(localStorage.getItem('accent')).toBeNull()
  })

  it('restores a stored accent on mount', () => {
    localStorage.setItem('accent', '#0d9488')
    render(<AccentPicker />)
    expect(document.documentElement.style.getPropertyValue('--primary')).toBe('#0d9488')
  })
})
