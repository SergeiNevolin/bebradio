import { expect, test, type Page } from '@playwright/test'

/**
 * Rooms + mashup queue.
 *
 * Covers the multi-source queue dispatcher end to end without touching
 * YouTube (no external network in CI): an uploaded mashup is queued from the
 * room's Mashups tab and via the raw queue API — including the
 * unknown-source rejection for future providers.
 */

const uniqueId = () => Math.random().toString(36).slice(2, 8)

/**
 * Builds a small PCM16 mono WAV in memory so the suite needs no binary
 * fixtures committed to the repo (same approach as mashup.spec.ts).
 * Default length is 60 s: the room's auto-advance eats tracks shortly after
 * duration + grace, and a 2 s tone would vanish mid-test.
 */
function makeToneWav(seconds = 60, freq = 440): Buffer {
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

async function registerAndGoRooms(page: Page, prefix: string) {
  const id = uniqueId()
  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(`${prefix}-${id}@e2e.test`)
  await page.getByPlaceholder('Username').fill(`${prefix}-${id}`)
  await page.getByPlaceholder('Password').fill('Test123!')
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')
  await page.goto('/rooms')
}

/** Uploads a tone WAV through the UI. */
async function uploadTone(page: Page, title: string, seconds = 60) {
  await page.goto('/mashup')
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
  await expect(page.getByText(title).first()).toBeVisible()
}

/** Waits until the uploaded card flips from processing to playable. */
async function waitUntilPlayable(page: Page, title: string) {
  await expect(
    page.getByRole('button', { name: `Play ${title}`, exact: true }).first(),
  ).toBeVisible({ timeout: 180000 })
}

/** Creates an open room through the UI and returns its code. */
async function createRoom(page: Page, name: string): Promise<string> {
  await page.goto('/rooms')
  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(name)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)
  const code = page.url().split('/room/')[1]
  expect(code).toBeTruthy()
  return code
}

/** Adds a mashup to the open room via the unified search in AddTrack. */
async function addMashupFromRoomTab(page: Page, title: string) {
  await page.getByPlaceholder('Search YouTube or bebradio...').fill(title)
  const row = page
    .locator('[class*="searchDropdown"] [class*="searchResult"]', { hasText: title })
    .first()
  await expect(row).toBeVisible({ timeout: 15000 })
  await row.getByRole('button', { name: 'Add' }).click()
}

interface ApiResult {
  status: number
  body: unknown
}

/** Same-origin fetch with the app's Bearer token from localStorage. */
async function apiCall(
  page: Page,
  method: string,
  path: string,
  data?: unknown,
): Promise<ApiResult> {
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

test.describe('Room queue from unified search', () => {
  test('queues an uploaded mashup with a Mashup badge and starts it', async ({ page }) => {
    const title = `E2E Queue ${uniqueId()}`
    await registerAndGoRooms(page, 'queue')
    await uploadTone(page, title, 8)
    await waitUntilPlayable(page, title)

    const code = await createRoom(page, `Queue Room ${uniqueId()}`)
    expect(page.url()).toContain(`/room/${code}`)
    await addMashupFromRoomTab(page, title)

    await expect(page.getByRole('heading', { name: 'Queue (1)' })).toBeVisible({
      timeout: 15000,
    })
    const item = page.locator('[class*="queueList"] > div', { hasText: title })
    await expect(item).toBeVisible()
    await expect(item.getByText('Mashup')).toBeVisible()

    // First track autostarts: the player resolves the library audio URL.
    const audio = page.locator('audio').first()
    await expect(audio).toHaveAttribute('src', /\/api\/tracks\/.+\/audio/, {
      timeout: 15000,
    })

    // Leave no playing track behind: short tones play out and the room drops
    // out of the home page "popular" list (other suites depend on it).
    await expect(page.getByText('No tracks yet. Add something above.')).toBeVisible({
      timeout: 60000,
    })
  })

  test('adding the same mashup twice stays idempotent', async ({ page }) => {
    const title = `E2E Dup ${uniqueId()}`
    await registerAndGoRooms(page, 'queuedup')
    await uploadTone(page, title, 8)
    await waitUntilPlayable(page, title)
    await createRoom(page, `Dup Room ${uniqueId()}`)

    await addMashupFromRoomTab(page, title)
    await expect(page.getByRole('heading', { name: 'Queue (1)' })).toBeVisible({
      timeout: 15000,
    })

    await addMashupFromRoomTab(page, title)
    await expect(page.getByText('Added!')).toBeVisible({ timeout: 15000 })
    await expect(page.getByRole('heading', { name: 'Queue (1)' })).toBeVisible()
    await expect(page.locator('[class*="queueList"] > div')).toHaveCount(1)

    // Leave no playing track behind (see above).
    await expect(page.getByText('No tracks yet. Add something above.')).toBeVisible({
      timeout: 60000,
    })
  })
})

/** Resolves ready library ids for the given titles (exact match). */
async function libraryIdsByTitle(
  page: Page,
  query: string,
  titles: string[],
): Promise<Map<string, string>> {
  const list = await apiCall(
    page,
    'GET',
    `/api/tracks/?sort=recent&limit=20&q=${encodeURIComponent(query)}`,
  )
  expect(list.status).toBe(200)
  const items = list.body as Array<{ id: string; title: string }>
  const byTitle = new Map<string, string>()
  for (const t of titles) {
    const found = items.find((item) => item.title === t)
    expect(found?.id).toBeTruthy()
    byTitle.set(t, found?.id as string)
  }
  return byTitle
}

const audioSrcFor = (id: string) =>
  new RegExp(`/api/tracks/${id.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/audio`)

test.describe('Queue playback chain', () => {
  test('advances through queued mashups automatically', async ({ page }) => {
    const suffix = uniqueId()
    const first = `E2E Chain1 ${suffix}`
    const second = `E2E Chain2 ${suffix}`
    await registerAndGoRooms(page, 'queuechain')
    await uploadTone(page, first, 3)
    await uploadTone(page, second, 8)
    await waitUntilPlayable(page, first)
    await waitUntilPlayable(page, second)
    const code = await createRoom(page, `Chain Room ${suffix}`)

    const ids = await libraryIdsByTitle(page, suffix, [first, second])
    const id1 = ids.get(first) as string
    const id2 = ids.get(second) as string

    const add1 = await apiCall(page, 'POST', `/api/rooms/${code}/queue`, {
      track_id: id1,
    })
    expect(add1.status).toBe(200)
    const add2 = await apiCall(page, 'POST', `/api/rooms/${code}/queue`, {
      track_id: id2,
    })
    expect(add2.status).toBe(200)

    // The short first track starts, ends, and the room moves to the second.
    const audio = page.locator('audio').first()
    await expect(audio).toHaveAttribute('src', audioSrcFor(id1), {
      timeout: 15000,
    })
    await expect(audio).toHaveAttribute('src', audioSrcFor(id2), {
      timeout: 30000,
    })
    await expect(page.getByRole('heading', { name: 'Queue (1)' })).toBeVisible()

    // Leave no playing track behind (see above): the 8 s tail plays out.
    await expect(page.getByText('No tracks yet. Add something above.')).toBeVisible({
      timeout: 60000,
    })
  })
})

test.describe('Skip and votes', () => {
  test('skip button and like/dislike votes drive the queue', async ({ page }) => {
    const suffix = uniqueId()
    const first = `E2E Vote1 ${suffix}`
    const second = `E2E Vote2 ${suffix}`
    await registerAndGoRooms(page, 'queuevote')
    await uploadTone(page, first)
    await uploadTone(page, second)
    await waitUntilPlayable(page, first)
    await waitUntilPlayable(page, second)
    const code = await createRoom(page, `Vote Room ${suffix}`)

    const ids = await libraryIdsByTitle(page, suffix, [first, second])
    const id1 = ids.get(first) as string
    const id2 = ids.get(second) as string
    expect((await apiCall(page, 'POST', `/api/rooms/${code}/queue`, { track_id: id1 })).status).toBe(200)
    expect((await apiCall(page, 'POST', `/api/rooms/${code}/queue`, { track_id: id2 })).status).toBe(200)

    const audio = page.locator('audio').first()
    await expect(audio).toHaveAttribute('src', audioSrcFor(id1), {
      timeout: 15000,
    })

    const likeBtn = page.getByRole('button', { name: 'Like this track', exact: true })
    const dislikeBtn = page.getByRole('button', { name: 'Dislike this track', exact: true })

    // Like: the count goes up, the track keeps playing.
    await likeBtn.click()
    await expect(likeBtn).toContainText('1', { timeout: 10000 })
    await expect(audio).toHaveAttribute('src', audioSrcFor(id1))

    // Unlike back to zero.
    await likeBtn.click()
    await expect(likeBtn).not.toContainText('1', { timeout: 10000 })

    // Skip: a solo listener is their own majority, the next track starts.
    await page.getByTitle('Vote to skip').click()
    await expect(audio).toHaveAttribute('src', audioSrcFor(id2), {
      timeout: 15000,
    })
    await expect(page.getByRole('heading', { name: 'Queue (1)' })).toBeVisible()

    // Dislike majority on the last track skips it and empties the queue.
    await dislikeBtn.click()
    await expect(page.getByText('Add a track to start listening together')).toBeVisible({
      timeout: 15000,
    })
    await expect(page.getByText('No tracks yet. Add something above.')).toBeVisible()
  })
})

test.describe('Unified search filters', () => {
  test('YouTube and bebradio filters narrow the result list', async ({ page }) => {
    const title = `E2E Filter ${uniqueId()}`
    await registerAndGoRooms(page, 'queuefilter')
    await uploadTone(page, title)
    await waitUntilPlayable(page, title)
    await createRoom(page, `Filter Room ${uniqueId()}`)

    await page.getByPlaceholder('Search YouTube or bebradio...').fill(title)
    const row = page
      .locator('[class*="searchDropdown"] [class*="searchResult"]', { hasText: title })
      .first()
    await expect(row).toBeVisible({ timeout: 15000 })

    await page.getByRole('button', { name: 'YouTube', exact: true }).click()
    await expect(row).not.toBeVisible()

    await page.getByRole('button', { name: 'bebradio', exact: true }).click()
    await expect(row).toBeVisible()

    await page.getByRole('button', { name: 'All', exact: true }).click()
    await expect(row).toBeVisible()
  })
})

test.describe('Queue API sources', () => {
  test('accepts library track_id and rejects unknown sources', async ({ page }) => {
    const title = `E2E API ${uniqueId()}`
    await registerAndGoRooms(page, 'queueapi')
    await uploadTone(page, title, 5)
    await waitUntilPlayable(page, title)
    const code = await createRoom(page, `API Room ${uniqueId()}`)

    const list = await apiCall(
      page,
      'GET',
      `/api/tracks/?sort=recent&limit=10&q=${encodeURIComponent(title)}`,
    )
    expect(list.status).toBe(200)
    const found = (list.body as Array<{ id: string; title: string }>).find(
      (t) => t.title === title,
    )
    expect(found?.id).toBeTruthy()

    const queued = await apiCall(page, 'POST', `/api/rooms/${code}/queue`, {
      track_id: found?.id,
    })
    expect(queued.status).toBe(200)
    const queuedBody = queued.body as { source: string; url: string; id: string }
    expect(queuedBody.source).toBe('upload')
    expect(queuedBody.url).toContain('/api/tracks/')
    expect(queuedBody.id).toBe(found?.id)

    const unsupported = await apiCall(page, 'POST', `/api/rooms/${code}/queue`, {
      source: 'spotify',
      url: 'https://open.spotify.com/track/abc',
    })
    expect(unsupported.status).toBe(400)

    const ghost = await apiCall(page, 'POST', `/api/rooms/${code}/queue`, {
      track_id: 'ghost',
    })
    expect(ghost.status).toBe(400)

    const room = await apiCall(page, 'GET', `/api/rooms/${code}`)
    expect(room.status).toBe(200)
    const queue = (room.body as { queue: Array<{ id: string }> }).queue
    expect(queue.map((t) => t.id)).toContain(found?.id)

    // Leave no playing track behind (see above): the 5 s tone plays out.
    await expect
      .poll(
        async () =>
          ((await apiCall(page, 'GET', `/api/rooms/${code}`)).body as {
            queue: unknown[]
          }).queue.length,
        { timeout: 60000 },
      )
      .toBe(0)
  })
})
