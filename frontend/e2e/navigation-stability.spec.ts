import { expect, test, type Page } from '@playwright/test'

/**
 * Navigation stability: one user journey across the whole service with a
 * mashup playing, plus anonymous deep links.
 *
 * What this catches that nothing else does:
 * - JS crashes on any route (pageerror listener — no other suite has one),
 * - broken API/static responses (>= 500, dead chunks, missing assets),
 * - dead nav links and broken back/forward entries,
 * - the global player dropping or restarting its audio between sections.
 *
 * Player transport itself (pause/next/shuffle/reload-restore) is covered by
 * player.spec.ts; room flows by room*.spec.ts. No YouTube anywhere: tracks
 * are in-memory tone WAVs like in the other suites.
 */

const uniqueId = () => Math.random().toString(36).slice(2, 8)

function makeToneWav(seconds = 30, freq = 440): Buffer {
  const rate = 22050
  const n = rate * seconds
  const data = Buffer.alloc(44 + n * 2)
  data.write('RIFF', 0)
  data.writeUInt32LE(36 + n * 2, 4)
  data.write('WAVE', 8)
  data.write('fmt ', 12)
  data.writeUInt32LE(16, 16)
  data.writeUInt16LE(1, 20) // PCM
  data.writeUInt16LE(1, 22) // mono
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

async function register(page: Page, prefix: string): Promise<string> {
  const id = uniqueId()
  const username = `${prefix}-${id}`
  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(`${prefix}-${id}@e2e.test`)
  await page.getByPlaceholder('Username').fill(username)
  await page.getByPlaceholder('Password').fill('Test123!')
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')
  return username
}

async function uploadTone(page: Page, title: string, seconds = 30, freq = 440) {
  await page.goto('/mashup')
  await page.getByRole('button', { name: 'Upload', exact: true }).click()

  const modal = page.locator('div.modal')
  await modal.locator('#mashup-file').setInputFiles({
    name: 'tone.wav',
    mimeType: 'audio/wav',
    buffer: makeToneWav(seconds, freq),
  })
  await modal.getByPlaceholder('Track title').fill(title)
  await modal.getByRole('button', { name: 'Upload', exact: true }).click()

  await expect(modal).toBeHidden({ timeout: 120000 })
  await expect(page.getByText(title).first()).toBeVisible()
}

async function waitUntilPlayable(page: Page, title: string) {
  await expect(
    page.getByRole('button', { name: `Play ${title}`, exact: true }).first(),
  ).toBeVisible({ timeout: 180000 })
}

async function playTone(page: Page, title: string): Promise<string> {
  await page
    .getByRole('button', { name: `Play ${title}`, exact: true })
    .first()
    .click()
  const audio = page.locator('audio').first()
  await expect(audio).toHaveAttribute('src', /\/api\/tracks\/.+\/audio/, {
    timeout: 15000,
  })
  await expect(async () => {
    const playing = await audio.evaluate(
      (el: HTMLAudioElement) => !el.paused && el.readyState >= 2,
    )
    expect(playing).toBe(true)
  }).toPass({ timeout: 30000 })
  return (await audio.getAttribute('src')) as string
}

/** Sidebar navigation (client-side, no reload); the mobile tab bar
 *  duplicates these links lower in the DOM, hence .first(). */
async function goSection(page: Page, path: string) {
  await page.locator(`nav a[href="${path}"]`).first().click()
  await expect(page).toHaveURL(path)
}

async function expectNoCrash(page: Page) {
  await expect(page.getByText('Page not found')).toHaveCount(0)
}

interface StabilityWatch {
  assertClean: () => Promise<void>
}

/** Fails the test at the end on any JS crash or broken backend/static
 *  response seen during the journey. Media aborts (seek/unmount) and the
 *  karaoke API (service off in e2e by design) are excluded. */
function watchStability(page: Page): StabilityWatch {
  const pageErrors: string[] = []
  const badNetwork: string[] = []
  page.on('pageerror', (err) => pageErrors.push(err.message))
  page.on('response', (res) => {
    const url = res.url()
    if (url.includes('/api/karaoke')) return
    const status = res.status()
    const kind = res.request().resourceType()
    if (status >= 500) {
      badNetwork.push(`${status} ${url}`)
    } else if (
      status === 404 &&
      (kind === 'document' || kind === 'script' || kind === 'stylesheet' || kind === 'font')
    ) {
      badNetwork.push(`404 ${kind} ${url}`)
    }
  })
  page.on('requestfailed', (req) => {
    if (req.resourceType() === 'media') return
    if (req.url().endsWith('/favicon.ico')) return
    if (req.url().includes('/api/karaoke')) return
    badNetwork.push(`failed ${req.url()} ${req.failure()?.errorText ?? ''}`)
  })
  return {
    assertClean: async () => {
      expect(pageErrors, `JS errors: ${pageErrors.join(' | ')}`).toEqual([])
      expect(badNetwork, `bad network: ${badNetwork.join(' | ')}`).toEqual([])
    },
  }
}

test.describe('Full journey without errors', () => {
  test('mashup keeps playing across every section, room, profile and back', async ({
    page,
  }) => {
    test.slow()
    const suffix = uniqueId()
    const title = `E2E Journey ${suffix}`
    const username = await register(page, 'journey')
    const watch = watchStability(page)

    // One long tone: the walk below must fit inside it, otherwise the
    // global player advances on end and the src-stability asserts below
    // would chase a moving target.
    await uploadTone(page, title, 120)
    await waitUntilPlayable(page, title)
    const src = await playTone(page, title)

    const audioSrc = () => page.locator('audio').first().getAttribute('src')
    const expectSameAudio = () => expect.poll(audioSrc, { timeout: 15000 }).toBe(src);

    // Home.
    await goSection(page, '/')
    await expect(page.getByRole('heading', { name: 'Комнаты' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Мешапы' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
    await expectNoCrash(page)
    await expectSameAudio()

    // Rooms list.
    await goSection(page, '/rooms')
    await expect(page.getByRole('heading', { name: 'Слушать музыку вместе с друзьями' })).toBeVisible()
    await expectNoCrash(page)
    await expectSameAudio()

    // Mashup catalog + full reload: the player restores the same track
    // from its snapshot even though the document (and chunks) reload.
    await goSection(page, '/mashup')
    await expect(page.getByRole('heading', { name: /все мешапы/i })).toBeVisible()
    await expectSameAudio()
    await page.reload()
    await expect(page.getByRole('heading', { name: /все мешапы/i })).toBeVisible({ timeout: 15000 })
    await expectNoCrash(page)
    await expectSameAudio()

    // Karaoke section (service off in e2e — only the shell must render).
    await goSection(page, '/karaoke')
    await expect(page.getByPlaceholder('Найти песню…')).toBeVisible({ timeout: 15000 })
    await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
    await expectNoCrash(page)
    await expectSameAudio()

    // A room of our own: header renders, shared bar hides (rooms have
    // their own эфир), audio element stays mounted.
    const roomName = `Journey Room ${suffix}`
    await goSection(page, '/rooms')
    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)
    await expect(page.getByRole('heading', { name: roomName })).toBeVisible({ timeout: 15000 })
    await expect(page.getByRole('button', { name: 'Expand player', exact: true })).toBeHidden()
    await expectNoCrash(page)

    // Back to the list (client-side popstate, no reload), then profile and
    // settings through the sidebar, then history both ways.
    await page.goBack()
    await expect(page).toHaveURL('/rooms')
    await page.locator('nav[aria-label="Разделы"] a', { hasText: 'Профиль' }).click()
    await expect(page).toHaveURL(/\/user\//)
    await expect(page.getByRole('heading', { name: username })).toBeVisible({ timeout: 15000 })
    await expectNoCrash(page)

    await page.locator('nav[aria-label="Разделы"] a', { hasText: 'Настройки' }).click()
    await expect(page).toHaveURL('/settings')
    await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()
    await expectNoCrash(page)

    await page.goBack()
    await expect(page).toHaveURL(/\/user\//)
    await page.goForward()
    await expect(page).toHaveURL('/settings')
    await expectNoCrash(page)

    await watch.assertClean()
  })
})

test.describe('Deep links resolve', () => {
  test('every section loads standalone without errors', async ({ page }) => {
    const watch = watchStability(page)

    await page.goto('/mashup')
    await expect(page.getByRole('heading', { name: /все мешапы/i })).toBeVisible({ timeout: 15000 })
    await expectNoCrash(page)

    await page.goto('/karaoke')
    await expect(page.getByPlaceholder('Найти песню…')).toBeVisible({ timeout: 15000 })
    await expectNoCrash(page)

    await page.goto('/rooms')
    await expect(page.getByRole('heading', { name: 'Слушать музыку вместе с друзьями' })).toBeVisible()
    await expectNoCrash(page)

    await page.goto('/login')
    await expect(page.getByRole('heading', { name: 'bebradio' })).toBeVisible()
    await expectNoCrash(page)

    await page.goto('/journey-does-not-exist')
    await expect(page.getByRole('heading', { name: '404' })).toBeVisible()

    await watch.assertClean()
  })
})
