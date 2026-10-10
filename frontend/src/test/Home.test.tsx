import { render, screen, fireEvent, act, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter, Routes, Route, Link, useLocation } from 'react-router-dom'
import Home from '../pages/Home'
import { GlobalPlayerBar, GlobalPlayerOverlays } from '../components/player/GlobalPlayerUI'
import { PlayerProvider } from '../context/PlayerContext'
import { useKaraoke } from '../karaoke/store'

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname}</div>
}

function HomeWithPlayer() {
  return (
    <MemoryRouter initialEntries={['/']}>
      <PlayerProvider>
        <LocationProbe />
        <Home />
        <GlobalPlayerBar />
        <GlobalPlayerOverlays />
      </PlayerProvider>
    </MemoryRouter>
  )
}

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: null }),
  AuthProvider: ({ children }: { children: React.ReactNode }) => children,
}))

function mockFetch(handler: (url: string, init?: RequestInit) => unknown) {
  globalThis.fetch = vi.fn((url: string, init?: RequestInit) =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve(handler(url, init)),
    }),
  ) as unknown as typeof fetch
}

const rooms = [
  { id: 'AAA111', name: 'Party', user_count: 5, track_count: 3, is_playing: true, has_password: false, auto_radio: false },
  { id: 'BBB222', name: 'Chill', user_count: 0, track_count: 0, is_playing: false, has_password: true, auto_radio: false },
  { id: 'STN001', name: 'Nonstop Hits', user_count: 10, track_count: 4, is_playing: true, has_password: false, auto_radio: true },
]

const tracks = [
  { id: 't1', title: 'Hit One', artist: 'DJ A', likes: 42, thumbnail: '', status: 'ready' },
  { id: 't2', title: 'Hit Two', artist: 'DJ B', likes: 7, thumbnail: '', status: 'ready' },
]

const karaokeSongs = [
  { id: 'song-a', title: 'Karaoke One', language: 'ru', duration: 185 },
  { id: 'song-b', title: 'Karaoke Two', language: 'en', duration: 240 },
]

function mockHomeApis(roomList = rooms, songList: unknown[] = karaokeSongs) {
  mockFetch((url) => {
    if (url.startsWith('/api/rooms')) return roomList
    if (url.startsWith('/api/tracks/')) return tracks
    if (url.startsWith('/api/karaoke/api/songs')) return { songs: songList }
    return []
  })
}

describe('Home', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockHomeApis()
    useKaraoke.getState().setSoundActive(false)
    // Снапшот плеера переживает размонтирование (persist при unmount) —
    // иначе трек прошлого теста ресторится в следующий.
    localStorage.removeItem('mashup-player')
  })

  it('shelves autodj rooms as popular stations with a badge', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Потоки')).toBeInTheDocument()
    expect(screen.getAllByText('Nonstop Hits').length).toBeGreaterThanOrEqual(1)
  })

  it('shows live rooms with a browse-all link', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Комнаты')).toBeInTheDocument()
    expect(screen.getAllByText('Party').length).toBeGreaterThanOrEqual(1)
  })

  it('hides the stations section when no room runs autodj', async () => {
    mockHomeApis(rooms.filter((r) => !r.auto_radio))
    render(<MemoryRouter><Home /></MemoryRouter>)
    await screen.findByText('Комнаты')
    expect(screen.queryByText('Потоки')).not.toBeInTheDocument()
  })

  it('shows live rooms with a browse-all link, stations excluded', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Комнаты')).toBeInTheDocument()
    expect(screen.getAllByText('Party').length).toBeGreaterThanOrEqual(1)
    expect(screen.queryByText('Chill')).not.toBeInTheDocument()
    const browseLinks = screen.getAllByText('Показать все')
      .filter((el) => el.closest('a')?.getAttribute('href') === '/rooms')
    expect(browseLinks.length).toBeGreaterThanOrEqual(1)
    for (const link of browseLinks) {
      expect(link.closest('a')).toHaveAttribute('href', '/rooms')
    }
  })

  it('shows top mashups with a link to the mashups page', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Мешапы')).toBeInTheDocument()
    expect(screen.getByText('Hit One')).toBeInTheDocument()
    const card = screen.getAllByTestId('top-track-card')[0]
    expect(card.textContent).toContain('DJ A')
    expect(card.textContent).toContain('42')
    expect(card.querySelector('svg')).not.toBeNull()
    const open = screen.getAllByText('Показать все')
      .find((el) => el.closest('a')?.getAttribute('href') === '/mashup')
    expect(open?.closest('a')).toHaveAttribute('href', '/mashup')
  })

  it('does not show how-it-works section', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(screen.queryByText('Как это работает')).not.toBeInTheDocument()
  })

  it('shelves karaoke songs with inline preview buttons', async () => {
    render(<MemoryRouter><Home /></MemoryRouter>)
    expect(await screen.findByText('Караоке')).toBeInTheDocument()
    const cards = await screen.findAllByTestId('karaoke-song-card')
    expect(cards.length).toBe(2)
    expect(cards[0]).toHaveAttribute('aria-label', 'Превью: Karaoke One')
    expect(screen.getByText('Karaoke One')).toBeInTheDocument()
    expect(screen.getByText('RU')).toBeInTheDocument()
    expect(screen.getByText('3:05')).toBeInTheDocument()
    const open = screen.getAllByText('Показать все')
      .find((el) => el.closest('a')?.getAttribute('href') === '/karaoke')
    expect(open?.closest('a')).toHaveAttribute('href', '/karaoke')
  })

  it('samples the karaoke shelf down to the limit deterministically', async () => {
    const many = Array.from({ length: 40 }, (_, i) => ({ id: `s${i}`, title: `Song ${i}` }))
    mockHomeApis(rooms, many)
    render(<MemoryRouter><Home /></MemoryRouter>)
    const cards = await screen.findAllByTestId('karaoke-song-card')
    expect(cards.length).toBe(20)
    expect(screen.getByText('Song 0')).toBeInTheDocument()
    expect(screen.queryByText('Song 6')).not.toBeInTheDocument()
  })

  it('opens karaoke preview panel without auto-play', async () => {
    render(<HomeWithPlayer />)
    const cards = await screen.findAllByTestId('karaoke-song-card')
    fireEvent.click(cards[0])
    const panel = await screen.findByTestId('karaoke-preview-panel')
    expect(panel).toHaveTextContent('Karaoke One')
    expect(screen.getByRole('button', { name: 'Слушать превью' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Спеть/ })).toBeInTheDocument()
    // Автоплея нет — нижней плашки тоже нет.
    expect(screen.queryByRole('button', { name: 'Expand player' })).not.toBeInTheDocument()
  })

  it('plays karaoke preview from the panel without a like button', async () => {
    render(<HomeWithPlayer />)
    const cards = await screen.findAllByTestId('karaoke-song-card')
    fireEvent.click(cards[0])
    await screen.findByTestId('karaoke-preview-panel')
    fireEvent.click(screen.getByRole('button', { name: 'Слушать превью' }))
    expect(await screen.findByRole('button', { name: 'Expand player' })).toBeInTheDocument()
    expect(screen.getByTestId('karaoke-preview-panel').className).toMatch(/withPlayer/)
    // Превью — не библиотечный трек: лайкать нечего.
    expect(screen.queryByRole('button', { name: 'Like' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Unlike' })).not.toBeInTheDocument()
  })

  it('sing button goes to karaoke and closes the panel', async () => {
    render(<HomeWithPlayer />)
    const cards = await screen.findAllByTestId('karaoke-song-card')
    fireEvent.click(cards[0])
    await screen.findByTestId('karaoke-preview-panel')
    fireEvent.click(screen.getByRole('button', { name: /Спеть/ }))
    expect(mockNavigate).toHaveBeenCalledWith('/karaoke')
    expect(screen.queryByTestId('karaoke-preview-panel')).not.toBeInTheDocument()
  })

  it('closes karaoke preview when a mashup starts playing', async () => {
    render(<HomeWithPlayer />)
    const kcards = await screen.findAllByTestId('karaoke-song-card')
    fireEvent.click(kcards[0])
    await screen.findByTestId('karaoke-preview-panel')
    const tcards = await screen.findAllByTestId('top-track-card')
    fireEvent.click(tcards[0])
    await waitFor(() => {
      expect(screen.queryByTestId('karaoke-preview-panel')).toBeNull()
    })
    expect(await screen.findByRole('button', { name: 'Expand player' })).toBeInTheDocument()
  })

  it('plays a mashup inline and shows the bottom player', async () => {
    render(<HomeWithPlayer />)
    const cards = await screen.findAllByTestId('top-track-card')
    expect(screen.queryByRole('button', { name: 'Expand player' })).not.toBeInTheDocument()
    fireEvent.click(cards[0])
    expect(await screen.findByRole('button', { name: 'Expand player' })).toBeInTheDocument()
  })

  it('opens Now Playing overlay without leaving home', async () => {
    render(<HomeWithPlayer />)
    const cards = await screen.findAllByTestId('top-track-card')
    fireEvent.click(cards[0])
    fireEvent.click(await screen.findByRole('button', { name: 'Expand player' }))
    expect(await screen.findByRole('dialog', { name: 'Now playing' })).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/')
  })

  it('opens the queue drawer on the home page', async () => {
    render(<HomeWithPlayer />)
    const cards = await screen.findAllByTestId('top-track-card')
    fireEvent.click(cards[0])
    await screen.findByRole('button', { name: 'Expand player' })
    expect(screen.queryByRole('complementary', { name: 'Now playing' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Queue' }))
    // Панель очереди ленивая — ждём подгрузку чанка.
    expect(await screen.findByRole('complementary', { name: 'Now playing' })).toBeInTheDocument()
  })

  it('keeps the player on the karaoke page and hides only while a karaoke song sings', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <PlayerProvider>
          <Routes>
            <Route
              path="/"
              element={
                <>
                  <Link to="/karaoke">go karaoke</Link>
                  <Home />
                </>
              }
            />
            <Route path="/karaoke" element={<div>karaoke page</div>} />
          </Routes>
          <GlobalPlayerBar />
          <GlobalPlayerOverlays />
        </PlayerProvider>
      </MemoryRouter>,
    )
    const cards = await screen.findAllByTestId('top-track-card')
    fireEvent.click(cards[0])
    await screen.findByRole('button', { name: 'Expand player' })
    fireEvent.click(screen.getByRole('button', { name: 'Queue' }))
    expect(screen.getByRole('complementary', { name: 'Now playing' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('link', { name: 'go karaoke' }))
    expect(await screen.findByText('karaoke page')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Expand player' })).toBeInTheDocument()

    act(() => useKaraoke.getState().setSoundActive(true))
    expect(screen.queryByRole('button', { name: 'Expand player' })).not.toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: 'Now playing' })).not.toBeInTheDocument()

    act(() => useKaraoke.getState().setSoundActive(false))
    expect(screen.getByRole('button', { name: 'Expand player' })).toBeInTheDocument()
  })

  it('hides the player and queue inside rooms', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <PlayerProvider>
          <Routes>
            <Route
              path="/"
              element={
                <>
                  <Link to="/room/ABC123">go room</Link>
                  <Home />
                </>
              }
            />
            <Route path="/room/:roomId" element={<div>room page</div>} />
          </Routes>
          <GlobalPlayerBar />
          <GlobalPlayerOverlays />
        </PlayerProvider>
      </MemoryRouter>,
    )
    const cards = await screen.findAllByTestId('top-track-card')
    fireEvent.click(cards[0])
    await screen.findByRole('button', { name: 'Expand player' })
    fireEvent.click(screen.getByRole('button', { name: 'Queue' }))
    expect(screen.getByRole('complementary', { name: 'Now playing' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('link', { name: 'go room' }))
    expect(await screen.findByText('room page')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Expand player' })).not.toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: 'Now playing' })).not.toBeInTheDocument()
  })

})
