import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import MobileTabBar from '../components/MobileTabBar'

function renderTabs(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <MobileTabBar />
    </MemoryRouter>,
  )
}

describe('MobileTabBar', () => {
  it('links to all four sections', () => {
    renderTabs()
    expect(screen.getByText('Главная').closest('a')).toHaveAttribute('href', '/')
    expect(screen.getByText('Комнаты').closest('a')).toHaveAttribute('href', '/rooms')
    expect(screen.getByText('Мэшапы').closest('a')).toHaveAttribute('href', '/mashup')
    expect(screen.getByText('Караоке').closest('a')).toHaveAttribute('href', '/karaoke')
  })

  it('marks the current section active', () => {
    const { container } = renderTabs('/mashup')
    const active = container.querySelectorAll('a[aria-current="page"]')
    expect(active).toHaveLength(1)
    expect(active[0]).toHaveTextContent('Мэшапы')
  })

  it('marks home active only on the exact root', () => {
    const { container } = renderTabs('/rooms')
    const active = container.querySelectorAll('a[aria-current="page"]')
    expect(active).toHaveLength(1)
    expect(active[0]).toHaveTextContent('Комнаты')
  })
})
