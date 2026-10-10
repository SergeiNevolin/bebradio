// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { useKaraoke } from '../store'
import { PlayerProvider, usePlayer } from '../../context/PlayerContext'
import { GlobalPlayerBar, GlobalPlayerOverlays, PanelDock } from '../../components/player/GlobalPlayerUI'

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname}</div>
}

function renderCatalog(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <PlayerProvider>
        <LocationProbe />
        <App />
        <GlobalPlayerBar />
        <GlobalPlayerOverlays />
      </PlayerProvider>
    </MemoryRouter>,
  )
}

let mockHasToken = false
vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  // Встроенный режим: BASE задан — видны гостевые ссылки на вход.
  return { ...actual, BASE: '/api/karaoke', hasAuthToken: () => mockHasToken }
})

vi.mock('../../context/AuthContext', () => ({
  useAuth: () => ({ user: null }),
}))

const mockShowToast = vi.fn()
vi.mock('../../context/ToastContext', () => ({
  useToast: () => ({ showToast: mockShowToast }),
}))

const META = {
  id: 't',
  title: 'Тестовая песня',
  audio: 'songs/t/minus.mp3',
  original: 'songs/t/original.mp3',
  vocals: 'songs/t/vocals.mp3',
  language: 'ru',
  lines: 1,
  duration: 40,
}

const META_EN = {
  ...META,
  id: 't2',
  title: 'Beta Song',
  language: 'en',
  lines: 5,
  duration: 200,
}

const META_A1 = { ...META, id: 'a1', title: 'ANNA ASTI - Uno', language: 'ru', lines: 10, duration: 100 }
const META_A2 = { ...META, id: 'a2', title: 'ANNA ASTI - Dos', language: 'ru', lines: 12, duration: 120 }
const META_C1 = { ...META, id: 'c1', title: 'CUEA - Track', language: 'en', lines: 8, duration: 90 }

function mockFetch() {
  vi.stubGlobal('fetch', (async (url: string) => {
    const u = String(url)
    const json = (data: unknown) => ({ ok: true, json: async () => data })
    if (u.includes('/api/songs')) return json({ songs: [META] })
    if (u.includes('lyrics.json')) {
      return json({
        language: 'ru',
        segments: [
          { start: 1, end: 3, text: 'раз два', words: [{ w: 'раз', s: 1, e: 2 }, { w: 'два', s: 2, e: 3 }] },
        ],
      })
    }
    if (u.includes('pitch.json')) return json({ t: [], midi: [] })
    if (u.includes('waveform.json')) return json({ peaks: [0.1, 0.2], duration: 40 })
    throw new Error(`unexpected fetch ${u}`)
  }) as unknown as typeof fetch)
}

beforeEach(() => {
  mockFetch()
  mockHasToken = false
  mockShowToast.mockClear()
  vi.spyOn(window.HTMLMediaElement.prototype, 'play').mockImplementation(async () => undefined)
  vi.spyOn(window.HTMLMediaElement.prototype, 'pause').mockImplementation(() => undefined)
  vi.stubGlobal('requestAnimationFrame', (cb: () => void) => setTimeout(cb, 16) as unknown as number)
  vi.stubGlobal('cancelAnimationFrame', (id: number) => clearTimeout(id))
  useKaraoke.setState({ screen: 'catalog', song: null, songs: [], favorites: [], recent: [], loadingSong: false, plays: {}, soundActive: false })
  localStorage.clear()
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('App: каталог → песня → назад', () => {
  it('возврат показывает каталог, а не чёрный экран', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    // каталог загрузился
    const sing = await screen.findByText('Петь')
    fireEvent.click(sing)
    // плеер открылся
    await screen.findByText('Просто подпевать')
    // назад в каталог
    fireEvent.click(screen.getByTitle('Вернуться в каталог'))
    // каталог снова виден (дожидаемся exit-анимации)
    await screen.findByRole('tab', { name: /Все песни/ }, { timeout: 3000 })
    // песня и в списке, и в полке «Продолжить петь»
    expect(screen.getAllByText('Тестовая песня').length).toBeGreaterThan(0)
    expect(document.body.textContent ?? '').toContain('Тестовая песня')
  })
})

describe('Catalog: шапка, табы, помощь', () => {
  it('показывает табы Все/Избранное с бейджами, таба Недавние нет', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    expect(screen.queryByRole('heading', { name: /Караоке/ })).toBeNull()
    expect(screen.getByRole('tab', { name: /Все песни/ }).textContent).toContain('1')
    expect(screen.getByRole('tab', { name: /Избранное/ }).textContent).toContain('0')
    // Недавнее живёт полкой, таба больше нет.
    expect(screen.queryByRole('tab', { name: /Недавние/ })).toBeNull()
  })

  it('пустое избранное зовёт ко всем песням', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    fireEvent.click(screen.getByRole('tab', { name: /Избранное/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Ко всем песням' }))
    await screen.findByText('Петь')
    // строка + Популярное
    expect(screen.getAllByText('Тестовая песня')).toHaveLength(2)
  })

  it('пустой поиск сбрасывается кнопкой', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    fireEvent.change(screen.getByLabelText('Найти песню'), { target: { value: 'zzz' } })
    fireEvent.click(await screen.findByRole('button', { name: 'Сбросить поиск' }))
    await screen.findByText('Петь')
    expect(screen.getAllByText('Тестовая песня')).toHaveLength(2)
  })
})

describe('Catalog: фаза 1 — полка, сортировка, язык, вид', () => {
  it('строки нумерованы, длительность справа', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    const nums = document.querySelectorAll('.num')
    expect(nums.length).toBe(1)
    expect(nums[0].textContent).toBe('1')
    expect(screen.getByText('0:40')).toBeInTheDocument()
  })

  it('полка «Недавние» остаётся при поиске — фильтруется только список', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    expect(screen.queryByRole('heading', { name: 'Недавние' })).toBeNull()
    act(() => {
      useKaraoke.getState().pushRecent('t')
    })
    expect(await screen.findByRole('heading', { name: 'Недавние' })).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Найти песню'), { target: { value: 'zzz' } })
    // полка на месте, а список ниже пуст с кнопкой сброса
    expect(screen.getByRole('heading', { name: 'Недавние' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сбросить поиск' })).toBeInTheDocument()
  })

  it('клик по карточке полки открывает превью, а не плеер', async () => {
    renderCatalog()
    await screen.findByText('Петь')
    act(() => {
      useKaraoke.getState().pushRecent('t')
    })
    await screen.findByRole('heading', { name: 'Недавние' })
    // строка + Популярное + Недавние
    const btns = screen.getAllByRole('button', { name: 'Превью: Тестовая песня' })
    expect(btns).toHaveLength(3)
    fireEvent.click(btns[1])
    const panel = await screen.findByTestId('karaoke-preview-panel')
    expect(panel).toHaveTextContent('Тестовая песня')
    expect(screen.queryByText('Просто подпевать')).toBeNull()
  })

  it('клик по строке открывает превью', async () => {
    const { container } = renderCatalog()
    await screen.findByText('Петь')
    const art = within(container.querySelector('.panel') as HTMLElement).getAllByRole('button', {
      name: /^Превью: /,
    })[0]
    fireEvent.click(art)
    const panel = await screen.findByTestId('karaoke-preview-panel')
    expect(panel).toHaveTextContent('Тестовая песня')
  })

  it('Спеть из панели на странице караоке открывает плеер сразу', async () => {
    renderCatalog('/karaoke')
    await screen.findByText('Петь')
    fireEvent.click(screen.getAllByRole('button', { name: 'Превью: Тестовая песня' })[0])
    await screen.findByTestId('karaoke-preview-panel')
    fireEvent.click(screen.getByRole('button', { name: /Спеть/ }))
    await screen.findByText('Просто подпевать')
    expect(screen.queryByTestId('karaoke-preview-panel')).not.toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/karaoke')
  })

  it('Спеть из панели вне караоке ведёт на /karaoke', async () => {
    renderCatalog('/')
    await screen.findByText('Петь')
    fireEvent.click(screen.getAllByRole('button', { name: 'Превью: Тестовая песня' })[0])
    await screen.findByTestId('karaoke-preview-panel')
    fireEvent.click(screen.getByRole('button', { name: /Спеть/ }))
    expect(await screen.findByTestId('location')).toHaveTextContent('/karaoke')
    expect(screen.queryByTestId('karaoke-preview-panel')).not.toBeInTheDocument()
  })

  it('сортировка меняет порядок строк секции', async () => {
    const { container } = render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    act(() => {
      useKaraoke.getState().setSongs([META, META_EN])
    })
    // Кнопки именно списка секции (полка «Популярное» живёт своим ранжиром).
    const sectionBtns = () =>
      within(container.querySelector('.panel') as HTMLElement).getAllByRole('button', { name: /^Превью: / })
    expect(sectionBtns()[0]).toHaveAttribute('aria-label', 'Превью: Тестовая песня')
    fireEvent.change(screen.getByLabelText('Сортировка'), { target: { value: 'dur-desc' } })
    expect(sectionBtns()[0]).toHaveAttribute('aria-label', 'Превью: Beta Song')
  })

  it('языковые чипсы фильтруют каталог и сбрасываются', async () => {
    const { container } = render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    expect(screen.queryByText('EN')).toBeNull()
    act(() => {
      useKaraoke.getState().setSongs([META, META_EN])
    })
    expect(await screen.findByText('EN')).toBeInTheDocument()
    fireEvent.click(screen.getByText('EN'))
    // Фильтр режет только список секции, полки показывают всё.
    const panel = () => container.querySelector('.panel') as HTMLElement
    expect(within(panel()).queryByText('Тестовая песня')).toBeNull()
    expect(within(panel()).getByText('Beta Song')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Все'))
    // строка + Популярное
    expect(screen.getAllByText('Тестовая песня')).toHaveLength(2)
  })

})

describe('Catalog: исполнители и популярность', () => {
  it('полка «Популярное» ранжирует по исполнениям', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    act(() => {
      useKaraoke.getState().setSongs([META, META_EN])
      useKaraoke.setState({ plays: { t2: 3 } })
    })
    const pop = screen.getByRole('heading', { name: 'Популярное' }).closest('section')
    expect(pop).not.toBeNull()
    const btns = within(pop as HTMLElement).getAllByRole('button', { name: /^Превью: / })
    expect(btns[0]).toHaveAttribute('aria-label', 'Превью: Beta Song')
  })

  it('спетая песня всплывает в «Популярном»', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    act(() => {
      useKaraoke.getState().setSongs([META, META_EN])
    })
    fireEvent.click(screen.getAllByText('Петь')[1])
    await screen.findByText('Просто подпевать')
    fireEvent.click(screen.getByTitle('Вернуться в каталог'))
    await screen.findByRole('tab', { name: /Все песни/ }, { timeout: 3000 })
    const pop = screen.getByRole('heading', { name: 'Популярное' }).closest('section')
    const btns = within(pop as HTMLElement).getAllByRole('button', { name: /^Превью: / })
    expect(btns[0]).toHaveAttribute('aria-label', 'Превью: Beta Song')
  })

  it('полка исполнителей ведёт на экран исполнителя и назад', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    act(() => {
      useKaraoke.getState().setSongs([META_A1, META_A2, META_C1])
    })
    expect(await screen.findByRole('heading', { name: 'Исполнители' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'ANNA ASTI' }))
    expect(screen.getByRole('button', { name: 'Каталог' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'ANNA ASTI' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Популярное' })).toBeInTheDocument()
    // чищенные названия в топе и в списке, слагов нет
    expect(screen.getAllByText('Uno')).toHaveLength(2)
    expect(screen.getAllByText('Dos')).toHaveLength(2)
    expect(screen.queryByText('ANNA ASTI - Uno')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Каталог' }))
    expect(await screen.findByRole('heading', { name: 'Исполнители' })).toBeInTheDocument()
  })

  it('секция показывает живое число песен', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    expect(screen.getByRole('heading', { name: /Все песни/ }).textContent).toContain('1 песня')
    fireEvent.change(screen.getByLabelText('Найти песню'), { target: { value: 'zzz' } })
    expect(screen.getByRole('heading', { name: /Все песни/ }).textContent).toContain('0')
  })

  it('гость видит хиро с регистрацией', async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    expect(screen.getByText('Регистрируйся и загружай своё')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Регистрация' })).toHaveAttribute('href', '/register')
    expect(screen.queryByText('Загрузи свою песню')).toBeNull()
  })

  it('вошедший видит хиро с загрузкой', async () => {
    mockHasToken = true
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>,
    )
    await screen.findByText('Петь')
    expect(screen.getByText('Загрузи свою песню')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Загрузить' })).toBeInTheDocument()
    expect(screen.queryByText('Регистрируйся и загружай своё')).toBeNull()
  })
})

describe('PanelDock: страница ужимается под панель', () => {
  function Opener() {
    const { setPreviewSong, previewSong, setQueueOpen, queueOpen } = usePlayer()
    return (
      <>
        <button
          type="button"
          onClick={() =>
            previewSong ? setPreviewSong(null) : setPreviewSong({ id: 't', title: 'T' })
          }
        >
          toggle-preview
        </button>
        <button type="button" onClick={() => setQueueOpen(!queueOpen)}>
          toggle-queue
        </button>
      </>
    )
  }

  function renderDocked(path = '/karaoke') {
    return render(
      <MemoryRouter initialEntries={[path]}>
        <PlayerProvider>
          <div className="app-root">
            <Opener />
            <PanelDock />
          </div>
        </PlayerProvider>
      </MemoryRouter>,
    )
  }

  it('ставит и снимает with-panel на app-root', async () => {
    renderDocked()
    const root = document.querySelector('.app-root') as HTMLElement
    expect(root.className).not.toContain('with-panel')
    fireEvent.click(screen.getByRole('button', { name: 'toggle-preview' }))
    expect(root.className).toContain('with-panel')
    fireEvent.click(screen.getByRole('button', { name: 'toggle-preview' }))
    expect(root.className).not.toContain('with-panel')
  })

  it('очередь тоже стыкует страницу', async () => {
    renderDocked('/')
    const root = document.querySelector('.app-root') as HTMLElement
    expect(root.className).not.toContain('with-panel')
    fireEvent.click(screen.getByRole('button', { name: 'toggle-queue' }))
    expect(root.className).toContain('with-panel')
    fireEvent.click(screen.getByRole('button', { name: 'toggle-queue' }))
    expect(root.className).not.toContain('with-panel')
  })
})
