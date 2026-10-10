import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import Shelf from '../components/media/Shelf'
import Hero from '../components/media/Hero'
import MediaCard from '../components/media/MediaCard'
import { TrackList, TrackRow } from '../components/media/TrackRows'

describe('Shelf', () => {
  it('renders title, count and show-all link', () => {
    render(
      <MemoryRouter>
        <Shelf title="Мешапы" count="42 песни" linkTo="/mashup">
          <span>card</span>
        </Shelf>
      </MemoryRouter>,
    )
    expect(screen.getByRole('heading', { name: 'Мешапы' })).toBeInTheDocument()
    expect(screen.getByText('42 песни')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Показать все' })).toHaveAttribute('href', '/mashup')
    expect(screen.getByRole('link', { name: 'Мешапы — Показать все' })).toHaveAttribute('href', '/mashup')
    expect(screen.getByText('card')).toBeInTheDocument()
  })

  it('shows skeletons while loading and empty node instead of the rail', () => {
    const { rerender } = render(
      <MemoryRouter>
        <Shelf title="Мешапы" loading>
          <span>card</span>
        </Shelf>
      </MemoryRouter>,
    )
    expect(screen.getByLabelText('Загрузка: Мешапы')).toBeInTheDocument()
    expect(screen.queryByText('card')).toBeNull()

    rerender(
      <MemoryRouter>
        <Shelf title="Мешапы" empty={<p>Пусто</p>}>
          <span>card</span>
        </Shelf>
      </MemoryRouter>,
    )
    expect(screen.getByText('Пусто')).toBeInTheDocument()
    expect(screen.queryByText('card')).toBeNull()
  })
})

describe('Hero', () => {
  it('renders title, sub, actions and stats', () => {
    render(
      <Hero
        title="Слушать вместе"
        sub="Подзаголовок"
        actions={<button type="button">Go</button>}
        stats={<span>42 rooms</span>}
      />,
    )
    expect(screen.getByText('Слушать вместе')).toBeInTheDocument()
    expect(screen.getByText('Подзаголовок')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Go' })).toBeInTheDocument()
    expect(screen.getByText('42 rooms')).toBeInTheDocument()
  })

  it('renders title only', () => {
    render(<Hero title="Только заголовок" />)
    expect(screen.getByText('Только заголовок')).toBeInTheDocument()
  })

  it('renders art slot', () => {
    const { container } = render(<Hero title="T" art={<img src="/karaoke-cat.gif" alt="Поющий кот" />} />)
    expect(screen.getByAltText('Поющий кот')).toBeInTheDocument()
    expect(container.querySelector('img[src="/karaoke-cat.gif"]')).not.toBeNull()
  })
})

describe('MediaCard', () => {
  it('works as a whole-card button with cover fallback', () => {
    const onToggle = vi.fn()
    render(
      <MediaCard
        asButton
        title="Hit One"
        meta="DJ A"
        tint="red"
        glyph="H"
        playLabel="Play Hit One"
        pressed={false}
        onToggle={onToggle}
        testId="card-1"
      />,
    )
    const card = screen.getByTestId('card-1')
    expect(card).toHaveAttribute('aria-label', 'Play Hit One')
    expect(card.textContent).toContain('H')
    fireEvent.click(card)
    expect(onToggle).toHaveBeenCalledTimes(1)
  })

  it('renders div mode with foot and equalizer while playing', () => {
    const { container } = render(
      <MediaCard
        title="Alpha"
        coverUrl="http://x/cover.jpg"
        active
        playing
        showEq
        playLabel="Pause Alpha"
        onToggle={() => {}}
        foot={<span>foot-row</span>}
        testId="card-2"
      />,
    )
    expect(screen.getByTestId('card-2').tagName).toBe('DIV')
    expect(screen.getByRole('button', { name: 'Pause Alpha' })).toBeInTheDocument()
    expect(screen.getByText('foot-row')).toBeInTheDocument()
    expect(container.querySelector('img[src="http://x/cover.jpg"]')).not.toBeNull()
  })

  it('defaults to the compact standard size, md opts out', () => {
    const { container, rerender } = render(
      <MediaCard title="Hit" playLabel="Play Hit" onToggle={() => {}} testId="card-sm" />,
    )
    expect(container.querySelector('.cardSm')).not.toBeNull()
    rerender(<MediaCard title="Hit" playLabel="Play Hit" onToggle={() => {}} testId="card-sm" size="md" />)
    expect(container.querySelector('.cardSm')).toBeNull()
  })

  it('disables toggle for not-ready tracks', () => {
    const onToggle = vi.fn()
    render(
      <MediaCard title="Raw" playLabel="Raw is not ready" onToggle={onToggle} disabled testId="card-3" />,
    )
    fireEvent.click(screen.getByTestId('card-3'))
    expect(onToggle).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Raw is not ready' })).toBeDisabled()
  })
})

describe('TrackRows', () => {
  it('renders panel rows with meta, badge and actions', () => {
    const onPlay = vi.fn()
    render(
      <TrackList>
        <TrackRow
          title="Вышка"
          meta="42 строк · 4:03 · RU"
          badge={<span>свой текст</span>}
          tint="blue"
          glyph="В"
          onPlay={onPlay}
          playLabel="Петь: Вышка"
          actions={<button type="button">Like</button>}
        />
      </TrackList>,
    )
    expect(screen.getByText('Вышка')).toBeInTheDocument()
    expect(screen.getByText('42 строк · 4:03 · RU')).toBeInTheDocument()
    expect(screen.getByText('свой текст')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Петь: Вышка' }))
    fireEvent.click(screen.getByText('Вышка'))
    expect(onPlay).toHaveBeenCalledTimes(2)
    expect(screen.getByRole('button', { name: 'Like' })).toBeInTheDocument()
  })

  it('shows spinner when busy and error text when failed', () => {
    render(
      <TrackList>
        <TrackRow title="X" busy error="Не открылась" onPlay={() => {}} playLabel="Петь: X" />,
      </TrackList>,
    )
    expect(screen.getByText('Не открылась')).toBeInTheDocument()
  })

  it('disables play affordances for not-ready tracks', () => {
    const onPlay = vi.fn()
    render(
      <TrackList>
        <TrackRow
          title="Raw"
          onPlay={onPlay}
          playLabel="Play Raw"
          disabled
          disabledLabel="Raw is not ready"
        />
      </TrackList>,
    )
    const art = screen.getByRole('button', { name: 'Raw is not ready' })
    expect(art).toBeDisabled()
    fireEvent.click(art)
    fireEvent.click(screen.getByText('Raw'))
    expect(onPlay).not.toHaveBeenCalled()
  })
})
