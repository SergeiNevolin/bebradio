import { expect, test, type Page, type Route } from '@playwright/test'

/**
 * Караоке (/karaoke) и его стык с общим плеером.
 *
 * karaoke-service в e2e-окружении выключен (профиль `karaoke` в
 * docker-compose.e2e.yml — исходников соседнего репо нет в CI), поэтому
 * каталог мокается через page.route на префиксе /api/karaoke/**:
 * манифест, lyrics.json и аудио. Это покрывает весь UI-контракт —
 * каталог, поиск, табы, превью, переход к пению и паузу общего плеера —
 * без GPU-пайплайна. Отдельный тест проверяет деградацию без моков:
 * страница не падает и показывает пустое состояние.
 */

interface MockSong {
  id: string
  title: string
  audio: string
  original?: string | null
  language?: string
  lines: number
  duration: number
  cover?: string | null
}

const SONGS: MockSong[] = [
  {
    id: 'e2e-song-1',
    title: 'E2E Singer - E2E Song One',
    audio: 'songs/e2e-song-1/minus.mp3',
    original: 'songs/e2e-song-1/original.mp3',
    language: 'ru',
    lines: 12,
    duration: 125,
  },
  {
    id: 'e2e-song-2',
    title: 'Another Artist - Second Song',
    audio: 'songs/e2e-song-2/minus.mp3',
    language: 'en',
    lines: 8,
    duration: 200,
  },
  {
    id: 'e2e-song-3',
    title: 'E2E Singer - Third Song',
    audio: 'songs/e2e-song-3/minus.mp3',
    language: 'ru',
    lines: 5,
    duration: 90,
  },
]

function lyricsFor(id: string) {
  return {
    language: 'ru',
    segments: [
      {
        start: 1,
        end: 3,
        text: 'раз два',
        words: [
          { w: 'раз', s: 1, e: 2 },
          { w: 'два', s: 2, e: 3 },
        ],
      },
    ],
    _id: id,
  }
}

/** In-memory tone WAV reused as fake karaoke audio for route fulfills. */
function makeToneWav(seconds = 5, freq = 440): Buffer {
  const rate = 22050
  const n = rate * seconds
  const data = Buffer.alloc(44 + n * 2)
  data.write('RIFF', 0)
  data.writeUInt32LE(36 + n * 2, 4)
  data.write('WAVE', 8)
  data.write('fmt ', 12)
  data.writeUInt32LE(16, 16)
  data.writeUInt16LE(1, 20)
  data.writeUInt16LE(1, 22)
  data.writeUInt32LE(rate, 24)
  data.writeUInt32LE(rate * 2, 28)
  data.writeUInt16LE(2, 32)
  data.writeUInt16LE(16, 34)
  data.write('data', 36)
  data.writeUInt32LE(n * 2, 40)
  for (let i = 0; i < n; i++) {
    data.writeInt16LE(
      Math.round(16000 * Math.sin((2 * Math.PI * freq * i) / rate)),
      44 + i * 2,
    )
  }
  return data
}

const json = (data: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(data),
})

/** Stands in for karaoke-service: manifest + lyrics + audio, rest 404. */
async function mockKaraokeService(page: Page, songs: MockSong[] = SONGS) {
  const tone = makeToneWav()
  await page.route('**/api/karaoke/**', async (route: Route) => {
    const path = new URL(route.request().url()).pathname.replace(
      /^\/api\/karaoke/,
      '',
    )
    if (path === '/api/songs') {
      await route.fulfill(json({ songs }))
      return
    }
    if (path.endsWith('/lyrics.json')) {
      const id = path.split('/')[2] ?? 'unknown'
      await route.fulfill(json(lyricsFor(id)))
      return
    }
    if (path.endsWith('.mp3')) {
      await route.fulfill({
        status: 200,
        contentType: 'audio/wav',
        body: tone,
      })
      return
    }
    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: '{}',
    })
  })
}

/**
 * Строка каталога (TrackRow): корень — div.group (единственное место с этим
 * классом), внутри — кнопка превью, «Петь» и избранное. Полки выше
 * («Популярное») содержат те же песни в MediaCard, поэтому все селекты
 * строк идут через этот скоуп, а не через page-wide getByRole.
 */
const songRow = (page: Page, title: string) =>
  page.locator('div.group').filter({ hasText: title })

const rowPreview = (page: Page, title: string) =>
  songRow(page, title).getByRole('button', { name: `Превью: ${title}` })

test.describe('Karaoke without mocks', () => {
  test('renders gracefully whether the service is up or not', async ({
    page,
  }) => {
    await page.goto('/karaoke')
    await expect(page.getByPlaceholder('Найти песню…')).toBeVisible({
      timeout: 15000,
    })
    await expect(
      page.getByRole('heading', { name: /Все песни/ }),
    ).toBeVisible()
    // Sidebar still switches sections (first match: the mobile tab bar
    // duplicates section links lower in the DOM).
    await page.locator('nav a[href="/mashups"]').first().click()
    await expect(page).toHaveURL('/mashups')
    await page.locator('nav a[href="/karaoke"]').first().click()
    await expect(page).toHaveURL('/karaoke')
    await expect(page.getByPlaceholder('Найти песню…')).toBeVisible()
  })

  test('home shelf shows songs or empty state, never a crash', async ({
    page,
  }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Караоке' })).toBeVisible()
    await expect(
      page
        .getByText('Песен пока нет')
        .or(page.getByTestId('karaoke-song-card').first()),
    ).toBeVisible({ timeout: 15000 })
  })
})

test.describe('Karaoke catalog (mocked service)', () => {
  test.beforeEach(async ({ page }) => {
    await mockKaraokeService(page)
  })

  test('lists songs, searches and filters by language', async ({ page }) => {
    await page.goto('/karaoke')
    await expect(songRow(page, 'E2E Song One')).toBeVisible({ timeout: 15000 })
    await expect(songRow(page, 'Second Song')).toBeVisible()

    // Search narrows the list (shelves above stay, rows are cut).
    await page.getByPlaceholder('Найти песню…').fill('Second')
    await expect(songRow(page, 'E2E Song One')).toBeHidden()
    await expect(songRow(page, 'Second Song')).toBeVisible()
    await expect(rowPreview(page, 'Second Song')).toBeVisible()
    // Reset only renders on empty results, so clear the input directly.
    await page.getByPlaceholder('Найти песню…').fill('')
    await expect(songRow(page, 'E2E Song One')).toBeVisible()

    // Language chips: EN leaves only the english song.
    await page.getByRole('button', { name: 'EN', exact: true }).click()
    await expect(songRow(page, 'Second Song')).toBeVisible()
    await expect(songRow(page, 'E2E Song One')).toBeHidden()
    await page.getByRole('button', { name: 'Все', exact: true }).click()
    await expect(songRow(page, 'E2E Song One')).toBeVisible()
  })

  test('favorites tab and artist drill-down', async ({ page }) => {
    await page.goto('/karaoke')
    await expect(songRow(page, 'E2E Song One')).toBeVisible({ timeout: 15000 })

    await songRow(page, 'E2E Song One')
      .getByRole('button', { name: 'В избранное' })
      .click()
    await page.getByRole('tab', { name: /Избранное/ }).click()
    await expect(songRow(page, 'E2E Song One')).toBeVisible()
    await page.getByRole('tab', { name: /Все песни/ }).click()

    // Artist card opens the artist page, back button returns.
    await page.getByRole('button', { name: 'E2E Singer' }).first().click()
    await expect(
      page.getByRole('heading', { name: 'E2E Singer' }),
    ).toBeVisible()
    await expect(page.getByText(/Популярное/)).toBeVisible()
    await page.getByRole('button', { name: 'Каталог' }).click()
    await expect(page.getByPlaceholder('Найти песню…')).toBeVisible()
  })

  test('row preview opens the side panel and closes', async ({ page }) => {
    await page.goto('/karaoke')
    await expect(songRow(page, 'E2E Song One')).toBeVisible({ timeout: 15000 })

    await rowPreview(page, 'E2E Song One').click()
    const panel = page.getByTestId('karaoke-preview-panel')
    await expect(panel).toBeVisible()
    await expect(panel.getByText('E2E Song One')).toBeVisible()
    await expect(
      panel.getByRole('button', { name: 'Слушать превью' }),
    ).toBeVisible()
    await expect(panel.getByRole('button', { name: 'Спеть' })).toBeVisible()

    await panel.getByRole('button', { name: 'Закрыть превью' }).click()
    await expect(panel).toBeHidden()
  })
})

test.describe('Karaoke session refresh', () => {
  test('expired access token silently refreshes, no relogin needed', async ({
    page,
  }) => {
    const uid = Math.random().toString(36).slice(2, 8)
    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(`refresh-${uid}@e2e.test`)
    await page.getByPlaceholder('Username').fill(`refresh-${uid}`)
    await page.getByPlaceholder('Password').fill('Test123!')
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    // Access протух (15 минут жизни), refresh-сессия в куке жива.
    // Караоке читает Bearer напрямую из localStorage.
    await page.evaluate(() => localStorage.setItem('token', 'expired-fake'))
    const staleAuth = 'Bearer expired-fake'

    // Сервис отвечает 401 только на протухший токен.
    await page.route('**/api/karaoke/api/songs*', async (route: Route) => {
      const auth = route.request().headers()['authorization']
      if (auth === staleAuth) {
        await route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'expired' }),
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ songs: [SONGS[0]] }),
      })
    })

    await page.locator('nav a[href="/karaoke"]').first().click()
    await expect(page).toHaveURL('/karaoke')
    // Каталог загрузился после молчаливого рефреша, а не «Требуется вход».
    await expect(songRow(page, 'E2E Song One')).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('Требуется вход в bebradio')).toBeHidden()
    // Токен в localStorage ротирован, сессия цела: настройки открываются.
    await expect(async () => {
      expect(await page.evaluate(() => localStorage.getItem('token'))).not.toBe('expired-fake')
    }).toPass({ timeout: 10000 })
    await page.goto('/settings')
    await expect(page).toHaveURL('/settings')
  })
})

test.describe('Karaoke shelf on home (mocked service)', () => {
  test.beforeEach(async ({ page }) => {
    await mockKaraokeService(page)
  })

  test('card opens preview, preview plays and leads to karaoke', async ({
    page,
  }) => {
    await page.goto('/')
    const card = page
      .getByTestId('karaoke-song-card')
      .filter({ hasText: 'E2E Song One' })
      .first()
    await expect(card).toBeVisible({ timeout: 15000 })
    await card.click()

    const panel = page.getByTestId('karaoke-preview-panel')
    await expect(panel).toBeVisible()

    await panel.getByRole('button', { name: 'Слушать превью' }).click()
    const audio = page.locator('audio').first()
    await expect(audio).toHaveAttribute('src', /\/api\/karaoke\//, {
      timeout: 15000,
    })
    await expect(async () => {
      const playing = await audio.evaluate(
        (el: HTMLAudioElement) => !el.paused && el.readyState >= 2,
      )
      expect(playing).toBe(true)
    }).toPass({ timeout: 30000 })

    await panel.getByRole('button', { name: 'Спеть' }).click()
    await expect(page).toHaveURL('/karaoke')
  })
})

test.describe('Karaoke singing vs global player', () => {
  test('starting a song pauses mashups and hides the bar until back', async ({
    page,
  }) => {
    // Real 30 s tone upload + transcode under parallel load.
    test.slow()
    const uid = Math.random().toString(36).slice(2, 8)
    const title = `E2E SingPause ${uid}`

    // Real mashup playing in the shared player.
    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(`sing-${uid}@e2e.test`)
    await page.getByPlaceholder('Username').fill(`sing-${uid}`)
    await page.getByPlaceholder('Password').fill('Test123!')
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await mockKaraokeService(page, [SONGS[0]])

    await page.goto('/mashups')
    await page.getByRole('button', { name: 'Upload', exact: true }).click()
    const modal = page.locator('div.modal')
    await modal.locator('#mashup-file').setInputFiles({
      name: 'tone.wav',
      mimeType: 'audio/wav',
      buffer: makeToneWav(30),
    })
    await modal.getByPlaceholder('Track title').fill(title)
    await modal.getByRole('button', { name: 'Upload', exact: true }).click()
    await expect(modal).toBeHidden({ timeout: 120000 })
    await expect(
      page.getByRole('button', { name: `Play ${title}`, exact: true }).first(),
    ).toBeVisible({ timeout: 180000 })
    await page
      .getByRole('button', { name: `Play ${title}`, exact: true })
      .first()
      .click()
    // Scoped by src: /karaoke opens its own <audio> elements later.
    const mashupAudio = page.locator('audio[src*="/api/tracks/"]')
    await expect(mashupAudio).toHaveAttribute(
      'src',
      /\/api\/tracks\/.+\/audio/,
      { timeout: 15000 },
    )

    // Open the song and start singing without a mic: the shared player
    // must pause and hide right away (countdown runs on user gesture).
    // Sidebar, not goto: only client-side navigation keeps the playing
    // mashup in the shared player (a reload would empty its list here).
    await page.locator('nav a[href="/karaoke"]').first().click()
    await expect(page).toHaveURL('/karaoke')
    await expect(songRow(page, 'E2E Song One')).toBeVisible({ timeout: 15000 })
    // Playback really survived the section switch.
    expect(
      await page
        .locator('audio[src*="/api/tracks/"]')
        .evaluate((el: HTMLAudioElement) => !el.paused),
    ).toBe(true)
    await songRow(page, 'E2E Song One')
      .getByRole('button', { name: 'Петь', exact: true })
      .click()
    await expect(
      page.getByRole('button', { name: 'Просто подпевать' }),
    ).toBeVisible({ timeout: 15000 })
    await page.getByRole('button', { name: 'Просто подпевать' }).click()

    await expect(
      page.getByRole('button', { name: 'Петь с оценкой' }),
    ).toBeHidden({ timeout: 10000 })
    await expect(
      page.getByRole('button', { name: 'Expand player', exact: true }),
    ).toBeHidden()
    await expect(async () => {
      const paused = await mashupAudio.evaluate(
        (el: HTMLAudioElement) => el.paused,
      )
      expect(paused).toBe(true)
    }).toPass({ timeout: 10000 })

    // Back to catalog returns the shared bar (same mashup, paused).
    // TopBar back during singing; the finished overlay's own button if
    // the short mocked song already ended underneath us.
    await page
      .getByRole('button', { name: 'Вернуться в каталог' })
      .or(page.getByRole('button', { name: 'Каталог', exact: true }))
      .first()
      .click()
    await expect(songRow(page, 'E2E Song One')).toBeVisible({ timeout: 15000 })
    await expect(
      page.getByRole('button', { name: 'Expand player', exact: true }),
    ).toBeVisible()
    await expect(mashupAudio).toHaveAttribute(
      'src',
      /\/api\/tracks\/.+\/audio/,
    )
  })
})
