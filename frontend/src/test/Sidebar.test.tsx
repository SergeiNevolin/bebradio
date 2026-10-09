import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Sidebar from '../components/Sidebar'

let mockUser: { id: string; username: string } | null = null
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: mockUser }),
}))

function renderSidebar(path = '/', props = {}) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Sidebar collapsed={false} overlay={false} open={false} onClose={vi.fn()} onNavigate={vi.fn()} {...props} />
    </MemoryRouter>,
  )
}

describe('Sidebar', () => {
  beforeEach(() => {
    mockUser = null
  })

  it('links to all sections for guests', () => {
    renderSidebar()
    expect(screen.getByRole('link', { name: 'Главная' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'Комнаты' })).toHaveAttribute('href', '/rooms')
    expect(screen.getByRole('link', { name: 'Мэшапы' })).toHaveAttribute('href', '/mashup')
    expect(screen.getByRole('link', { name: 'Караоке' })).toHaveAttribute('href', '/karaoke')
    expect(screen.queryByRole('link', { name: 'Профиль' })).toBeNull()
  })

  it('shows profile and settings for signed-in users', () => {
    mockUser = { id: 'u1', username: 'me' }
    renderSidebar()
    expect(screen.getByRole('link', { name: 'Профиль' })).toHaveAttribute('href', '/user/u1')
    expect(screen.getByRole('link', { name: 'Настройки' })).toHaveAttribute('href', '/settings')
  })

  it('marks the current section active', () => {
    const { container } = renderSidebar('/mashup')
    const active = screen.getByRole('link', { name: 'Мэшапы' })
    expect(active.className).toMatch(/itemActive/)
    expect(container.querySelectorAll('[class*="itemActive"]')).toHaveLength(1)
  })

  it('renders nothing when the overlay drawer is closed', () => {
    const { container } = renderSidebar('/karaoke', { overlay: true, open: false })
    expect(container.querySelector('nav')).toBeNull()
  })

  it('closes the drawer on backdrop click and notifies on navigate', () => {
    const onClose = vi.fn()
    const onNavigate = vi.fn()
    renderSidebar('/karaoke', { overlay: true, open: true, onClose, onNavigate })
    expect(screen.getByRole('navigation', { name: 'Разделы' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('link', { name: 'Главная' }))
    expect(onNavigate).toHaveBeenCalledTimes(1)
  })
})
