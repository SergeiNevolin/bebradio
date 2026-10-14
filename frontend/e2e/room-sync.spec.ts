import { expect, test, type Browser, type Page } from '@playwright/test'

/**
 * Multi-client room sync and resilience.
 *
 * Everything here needs two independent browser contexts (two users):
 * shared playback, cross-client chat, WS reconnect after a network loss,
 * and the concurrent double-add race. Single-client queue behavior is
 * covered by room-queue-mashup.spec.ts; voting lives in vote.spec.ts
 * (currently disabled — it depends on YouTube).
 *
 * Conventions follow the other suites: short self-made tones (no external
 * network in CI), unique titles per test, and no playing track left behind
 * (short tones drain on their own so the home "popular" list stays quiet).
 */

const BASE = process.env.E2E_BASE_URL || 'http://127.0.0.1:3000'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

/** Builds a small PCM16 mono WAV in memory — no binary fixtures in repo. */
function makeToneWav(seconds = 8, freq = 440): Buffer {
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
  await page.goto(`${BASE}/register`)
  await page.getByPlaceholder('Email').fill(`${prefix}-${id}@e2e.test`)
  await page.getByPlaceholder('Username').fill(username)
  await page.getByPlaceholder('Password').fill('Test123!')
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL(`${BASE}/`)
  return username
}

async function uploadTone(page: Page, title: string, seconds = 8) {
  await page.goto(`${BASE}/mashup`)
  await page.getByRole('button', { name: 'Upload', exact: true }).click()

  const modal = page.locator('div.modal')
  await modal.locator('#mashup-file').setInputFiles({
    name: 'tone.wav',
    mimeType: 'audio/wav',
    buffer: makeToneWav(seconds),
  })
  await modal.getByPlaceholder('Track title').fill(title)
  await modal.getByRole('button', { name: 'Upload', exact: true }).click()

  await expect(modal).toBeHidden({ timeout: 120000 })
  await expect(
    page.getByRole('button', { name: `Play ${title}`, exact: true }).first(),
  ).toBeVisible({ timeout: 120000 })
}

/** Same-origin fetch with the page's Bearer token from localStorage. */
async function apiCall(page: Page, method: string, path: string, data?: unknown) {
  return page.evaluate(
    async ({ method: innerMethod, path: innerPath, data: innerData }) => {
      const token = localStorage.getItem('token')
      const res = await fetch(innerPath, {
        method: innerMethod,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: innerData === undefined ? undefined : JSON.stringify(innerData),
      })
      let body: unknown = null
      try {
        body = await res.json()
      } catch {
        /* non-JSON body */
      }
      return { status: res.status, body }
    },
    { method, path, data },
  )
}

async function libraryIdByTitle(page: Page, query: string, title: string): Promise<string> {
  const list = await apiCall(page, 'GET', `/api/tracks/?sort=recent&limit=20&q=${encodeURIComponent(query)}`)
  expect(list.status).toBe(200)
  const found = (list.body as Array<{ id: string; title: string }>).find((t) => t.title === title)
  expect(found?.id).toBeTruthy()
  return found?.id as string
}

async function createRoom(page: Page, name: string): Promise<string> {
  await page.goto(`${BASE}/rooms`)
  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(name)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)
  const code = page.url().split('/room/')[1]
  expect(code).toBeTruthy()
  return code
}

async function audioSrc(page: Page): Promise<string | null> {
  return page.locator('audio').first().getAttribute('src')
}

async function audioTime(page: Page): Promise<number> {
  return page
    .locator('audio')
    .first()
    .evaluate((el) => (el as HTMLAudioElement).currentTime)
}

/** Second user in an isolated context joins the room by code. */
async function joinAsSecondUser(browser: Browser, prefix: string, code: string): Promise<{ ctx: import('@playwright/test').BrowserContext; page: Page }> {
  const ctx = await browser.newContext()
  const page = await ctx.newPage()
  await register(page, prefix)
  await page.goto(`${BASE}/room/${code}`)
  // Audio element resolves the library URL: WS connected + state loaded.
  await expect.poll(async () => audioSrc(page), { timeout: 20000 }).toMatch(/\/api\/tracks\/.+\/audio/)
  return { ctx, page }
}

/** Both decks currently resolve the same non-empty audio URL. */
async function expectSameAudio(pageA: Page, pageB: Page, timeout = 20000) {
  await expect
    .poll(
      async () => {
        const [a, b] = await Promise.all([audioSrc(pageA), audioSrc(pageB)])
        return a !== null && a === b ? a : null
      },
      { timeout },
    )
    .toMatch(/\/api\/tracks\/.+\/audio/)
}

test.describe('Two listeners share one room', () => {
  test('play the same track and exchange chat', async ({ browser }) => {
    test.slow()
    const suffix = uniqueId()
    const first = `E2E Sync1 ${suffix}`
    const second = `E2E Sync2 ${suffix}`

    const ctxA = await browser.newContext()
    const pageA = await ctxA.newPage()
    await register(pageA, 'sync')
    await uploadTone(pageA, first)
    await uploadTone(pageA, second)
    const code = await createRoom(pageA, `Sync Room ${suffix}`)

    const id1 = await libraryIdByTitle(pageA, suffix, first)
    const id2 = await libraryIdByTitle(pageA, suffix, second)
    expect((await apiCall(pageA, 'POST', `/api/rooms/${code}/queue`, { track_id: id1 })).status).toBe(200)
    expect((await apiCall(pageA, 'POST', `/api/rooms/${code}/queue`, { track_id: id2 })).status).toBe(200)

    const { ctx: ctxB, page: pageB } = await joinAsSecondUser(browser, 'syncguest', code)
    try {
      // Both decks resolve the same library audio URL.
      await expectSameAudio(pageA, pageB)

      // Chat flows both ways through the room broadcast.
      const msgFromA = `sync-a-${suffix}`
      await pageA.getByPlaceholder('Type a message...').fill(msgFromA)
      await pageA.getByRole('button', { name: 'Send' }).click()
      await expect(pageB.getByText(msgFromA)).toBeVisible({ timeout: 10000 })

      const msgFromB = `sync-b-${suffix}`
      await pageB.getByPlaceholder('Type a message...').fill(msgFromB)
      await pageB.getByRole('button', { name: 'Send' }).click()
      await expect(pageA.getByText(msgFromB)).toBeVisible({ timeout: 10000 })

      // Both tones drain on their own; nothing keeps playing afterwards.
      await expect(pageA.getByText('No tracks yet. Add something above.')).toBeVisible({
        timeout: 60000,
      })
    } finally {
      await ctxB.close()
      await ctxA.close()
    }
  })

  test('survives a connection loss without rewinding', async ({ browser }) => {
    test.slow()
    const suffix = uniqueId()
    const first = `E2E Reconn1 ${suffix}`
    const second = `E2E Reconn2 ${suffix}`

    const ctxA = await browser.newContext()
    const pageA = await ctxA.newPage()
    await register(pageA, 'reconn')
    await uploadTone(pageA, first)
    await uploadTone(pageA, second)
    const code = await createRoom(pageA, `Reconn Room ${suffix}`)
    const id1 = await libraryIdByTitle(pageA, suffix, first)
    const id2 = await libraryIdByTitle(pageA, suffix, second)
    expect((await apiCall(pageA, 'POST', `/api/rooms/${code}/queue`, { track_id: id1 })).status).toBe(200)
    expect((await apiCall(pageA, 'POST', `/api/rooms/${code}/queue`, { track_id: id2 })).status).toBe(200)
    await expectSameAudio(pageA, pageA)

    const { ctx: ctxB, page: pageB } = await joinAsSecondUser(browser, 'reconnguest', code)
    try {
      await expectSameAudio(pageA, pageB)
      const t0 = await audioTime(pageB)

      // Drop the network: WS dies, the client retries on its timer.
      await ctxB.setOffline(true)
      await pageB.waitForTimeout(6000)
      await ctxB.setOffline(false)

      // Same track on both sides again — no restart, no foreign track.
      await expectSameAudio(pageA, pageB, 25000)

      // No rewind through the outage (2 s tolerance for slew/crossfade).
      const t1 = await audioTime(pageB)
      expect(t1).toBeGreaterThanOrEqual(t0 - 2)

      // And the room still talks to the recovered client.
      const msg = `reconn-ok-${suffix}`
      await pageB.getByPlaceholder('Type a message...').fill(msg)
      await pageB.getByRole('button', { name: 'Send' }).click()
      await expect(pageA.getByText(msg)).toBeVisible({ timeout: 10000 })

      // Both tones drain on their own afterwards.
      await expect(pageA.getByText('No tracks yet. Add something above.')).toBeVisible({
        timeout: 60000,
      })
    } finally {
      await ctxB.close()
      await ctxA.close()
    }
  })
})

test.describe('Queue race safety', () => {
  test('concurrent double add stays a single entry', async ({ browser }) => {
    test.slow()
    const suffix = uniqueId()
    const title = `E2E Race ${suffix}`

    const ctxA = await browser.newContext()
    const pageA = await ctxA.newPage()
    try {
      await register(pageA, 'race')
      await uploadTone(pageA, title)
      const code = await createRoom(pageA, `Race Room ${suffix}`)
      const id = await libraryIdByTitle(pageA, suffix, title)

      // Two POSTs in flight at once — the server serializes check+append.
      const [r1, r2] = await Promise.all([
        apiCall(pageA, 'POST', `/api/rooms/${code}/queue`, { track_id: id }),
        apiCall(pageA, 'POST', `/api/rooms/${code}/queue`, { track_id: id }),
      ])
      expect(r1.status).toBe(200)
      expect(r2.status).toBe(200)

      const room = await apiCall(pageA, 'GET', `/api/rooms/${code}`)
      expect(room.status).toBe(200)
      const queue = (room.body as { queue: Array<{ id: string }> }).queue
      expect(queue.filter((t) => t.id === id)).toHaveLength(1)

      await pageA.goto(`${BASE}/room/${code}`)
      await expect(pageA.getByRole('heading', { name: 'Queue (1)' })).toBeVisible({
        timeout: 15000,
      })
      await expect(pageA.getByText('No tracks yet. Add something above.')).toBeVisible({
        timeout: 60000,
      })
    } finally {
      await ctxA.close()
    }
  })
})
