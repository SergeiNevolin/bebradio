// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PlayerProvider, usePlayer } from '../context/PlayerContext'
import { GlobalPlayerOverlays } from '../components/player/GlobalPlayerUI'
import { useKaraoke } from '../karaoke/store'
import type { KaraokePreview } from '../lib/karaokePreview'

let mockUser: { id: string; username: string } | null = null
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: mockUser }),
}))

const mockShowToast = vi.fn()
vi.mock('../context/ToastContext', () => ({
  useToast: () => ({ showToast: mockShowToast }),
}))

const PREVIEW: KaraokePreview = {
  id: 't',
  title: 'Песня',
  language: 'ru',
  duration: 40,
  owner_id: 'u1',
  owner_name: 'Биба',
}

function Opener({ preview = PREVIEW }: { preview?: KaraokePreview | null }) {
  const { setPreviewSong } = usePlayer()
  return (
    <button type="button" onClick={() => setPreviewSong(preview)}>
      open
    </button>
  )
}

function renderPanel(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <PlayerProvider>
        <Opener />
        <GlobalPlayerOverlays />
      </PlayerProvider>
    </MemoryRouter>,
  )
}

function openPanel() {
  fireEvent.click(screen.getByRole('button', { name: 'open' }))
  return screen.findByTestId('karaoke-preview-panel')
}

beforeEach(() => {
  mockUser = null
  mockShowToast.mockClear()
  vi.spyOn(window.HTMLMediaElement.prototype, 'play').mockImplementation(async () => undefined)
  vi.spyOn(window.HTMLMediaElement.prototype, 'pause').mockImplementation(() => undefined)
  useKaraoke.setState({
    screen: 'catalog',
    song: null,
    songs: [],
    favorites: [],
    recent: [],
    loadingSong: false,
    plays: {},
    soundActive: false,
    editRequest: null,
    editorOpen: false,
  })
  localStorage.clear()
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('KaraokePreviewPanel: владелец', () => {
  it('гость видит загрузчика, но не кнопки', async () => {
    renderPanel()
    await openPanel()
    expect(screen.getByText('Загрузил Биба')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Изменить' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Удалить' })).toBeNull()
  })

  it('чужой не видит кнопок', async () => {
    mockUser = { id: 'u2', username: 'Чужой' }
    renderPanel()
    await openPanel()
    expect(screen.getByText('Загрузил Биба')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Изменить' })).toBeNull()
  })

  it('легаси без владельца правит любой вошедший', async () => {
    mockUser = { id: 'u2', username: 'Чужой' }
    render(
      <MemoryRouter initialEntries={['/']}>
        <PlayerProvider>
          <Opener preview={{ ...PREVIEW, owner_id: null, owner_name: null }} />
          <GlobalPlayerOverlays />
        </PlayerProvider>
      </MemoryRouter>,
    )
    await openPanel()
    expect(screen.queryByText(/Загрузил/)).toBeNull()
    expect(screen.getByRole('button', { name: 'Изменить' })).toBeInTheDocument()
  })

  it('Изменить открывает модалку меты и сохраняет название', async () => {
    mockUser = { id: 'u1', username: 'Биба' }
    const calls: Array<{ url: string; init?: RequestInit }> = []
    vi.stubGlobal(
      'fetch',
      (async (url: string, init?: RequestInit) => {
        calls.push({ url: String(url), init })
        return { ok: true, json: async () => ({}) }
      }) as unknown as typeof fetch,
    )
    renderPanel('/karaoke')
    await openPanel()
    fireEvent.click(screen.getByRole('button', { name: 'Изменить' }))
    // Модалка как в редакторе мешапа, панель при этом не закрывается.
    expect(screen.getByRole('dialog', { name: 'Настройки караоке' })).toBeInTheDocument()
    expect(screen.getByTestId('karaoke-preview-panel')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Название песни'), { target: { value: 'Новое название' } })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Сохранено' })).toBeInTheDocument()
    })
    expect(
      calls.some((c) => c.url.endsWith('/api/songs/t/meta') && c.init?.method === 'PUT'),
    ).toBe(true)
  })

  it('Редактировать караоке ставит заявку и закрывает панель', async () => {
    mockUser = { id: 'u1', username: 'Биба' }
    renderPanel('/karaoke')
    await openPanel()
    fireEvent.click(screen.getByRole('button', { name: 'Редактировать караоке' }))
    expect(useKaraoke.getState().editRequest).toBe('t')
    expect(screen.queryByTestId('karaoke-preview-panel')).not.toBeInTheDocument()
  })

  it('Удалить зовёт API и чистит стор', async () => {
    mockUser = { id: 'u1', username: 'Биба' }
    useKaraoke.getState().setSongs([
      { id: 't', title: 'Песня', audio: 'x', lines: 1, duration: 40 },
    ])
    useKaraoke.getState().toggleFavorite('t')
    const calls: Array<{ url: string; init?: RequestInit }> = []
    vi.stubGlobal(
      'fetch',
      (async (url: string, init?: RequestInit) => {
        calls.push({ url: String(url), init })
        return { ok: true, json: async () => ({}) }
      }) as unknown as typeof fetch,
    )
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPanel('/karaoke')
    await openPanel()
    fireEvent.click(screen.getByRole('button', { name: 'Удалить' }))
    await waitFor(() => {
      expect(mockShowToast).toHaveBeenCalledWith('Песня удалена', 'success')
    })
    expect(calls.some((c) => c.url.endsWith('/api/songs/t') && c.init?.method === 'DELETE')).toBe(true)
    expect(useKaraoke.getState().songs).toEqual([])
    expect(useKaraoke.getState().favorites).toEqual([])
    expect(screen.queryByTestId('karaoke-preview-panel')).not.toBeInTheDocument()
  })

  it('отмена confirm ничего не трогает', async () => {
    mockUser = { id: 'u1', username: 'Биба' }
    vi.stubGlobal('fetch', (async () => {
      throw new Error('no fetch expected')
    }) as unknown as typeof fetch)
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderPanel('/karaoke')
    await openPanel()
    fireEvent.click(screen.getByRole('button', { name: 'Удалить' }))
    expect(screen.getByTestId('karaoke-preview-panel')).toBeInTheDocument()
  })
})
