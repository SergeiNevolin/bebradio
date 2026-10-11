import { expect, test, type Page } from '@playwright/test'

/**
 * Global mashup player: one <audio> mounted in App, playback must survive
 * navigation between sections (/mashup, /, /karaoke), hide (not unmount) in
 * rooms and while karaoke is singing, and restore after reload.
 *
 * Real backend + music-service, no YouTube: tracks are in-memory tone WAVs
 * uploaded through the UI (same approach as mashup.spec.ts). 30 s is long
 * enough that no test outlives the track (no auto-advance surprises) yet
 * short enough to transcode fast.
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

async function register(page: Page, prefix: string) {
  const id = uniqueId()
  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(`${prefix}-${id}@e2e.test`)
  await page.getByPlaceholder('Username').fill(`${prefix}-${id}`)
  await page.getByPlaceholder('Password').fill('Test123!')
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')
}

async function uploadTone(page: Page, title: string, freq = 440) {
  await page.goto('/mashup')
  await page.getByRole('button', { name: 'Upload', exact: true }).click()

  const modal = page.locator('div.modal')
  await modal.locator('#mashup-file').setInputFiles({
    name: 'tone.wav',
    mimeType: 'audio/wav',
    buffer: makeToneWav(30, freq),
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
  // Stable playback: currentTime moving means play() fully resolved, so a
  // later pause() (room entry, karaoke) can't be undone by its resolution.
  await expect(async () => {
    const t = await audio.evaluate((el: HTMLAudioElement) => el.currentTime)
    expect(t).toBeGreaterThan(1)
  }).toPass({ timeout: 15000 })
  return (await audio.getAttribute('src')) as string
}

const audioPlaying = (page: Page) =>
  page.locator('audio').first().evaluate(
    (el: HTMLAudioElement) => !el.paused && el.readyState >= 2,
  )

/**
 * Section switching the way users do it: sidebar links use client-side
 * routing (no document reload), so the mounted <audio> and its state must
 * survive. (page.goto() always fully reloads — that path is covered by the
 * reload-restore test instead.) The mobile tab bar duplicates these links
 * lower in the DOM, hence .first().
 */
async function goSection(page: Page, path: string) {
  await page.locator(`nav a[href="${path}"]`).first().click()
  await expect(page).toHaveURL(path)
}

test.describe('Global player across sections', () => {
  test('keeps playing while switching mashup / home / karaoke', async ({
    page,
  }) => {
    // 30 s tone upload + transcode under parallel load.
    test.slow()
    const title = `E2E Nav ${uniqueId()}`
    await register(page, 'playnav')
    await uploadTone(page, title)
    await waitUntilPlayable(page, title)
    const src = await playTone(page, title)

    // Home: the bar stays, same track, time moves.
    await goSection(page, '/')
    await expect(
      page.getByRole('button', { name: 'Pause', exact: true }),
    ).toBeVisible()
    await expect(page.locator('audio').first()).toHaveAttribute('src', src)
    const t1 = await page
      .locator('audio')
      .first()
      .evaluate((el: HTMLAudioElement) => el.currentTime)
    await page.waitForTimeout(1500)
    const t2 = await page
      .locator('audio')
      .first()
      .evaluate((el: HTMLAudioElement) => el.currentTime)
    expect(t2).toBeGreaterThan(t1)

    // Karaoke (not singing): the shared bar is still there, same audio.
    await goSection(page, '/karaoke')
    await expect(
      page.getByRole('button', { name: 'Pause', exact: true }),
    ).toBeVisible()
    await expect(page.locator('audio').first()).toHaveAttribute('src', src)

    // Back to mashup through the sidebar: still the same track.
    await goSection(page, '/mashup')
    await expect(page.locator('audio').first()).toHaveAttribute('src', src)
    expect(await audioPlaying(page)).toBe(true)
  })

  test('hides in rooms but keeps the audio, returns after leaving', async ({
    page,
  }) => {
    // 30 s tone upload + transcode under parallel load.
    test.slow()
    const title = `E2E RoomHide ${uniqueId()}`
    await register(page, 'playroom')
    await uploadTone(page, title)
    await waitUntilPlayable(page, title)
    const src = await playTone(page, title)

    // Create a room: rooms have their own эфир (and pause the shared
    // player on entry), the mashup bar hides. Sidebar, not goto: a full
    // reload would drop the walkable list (rooms don't register mashups).
    const roomName = `Player Room ${uniqueId()}`
    await goSection(page, '/rooms')
    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    // Bar UI gone (Expand/Queue), the element stays mounted and paused.
    await expect(
      page.getByRole('button', { name: 'Expand player', exact: true }),
    ).toBeHidden()
    await expect(
      page.getByRole('button', { name: 'Queue', exact: true }),
    ).toBeHidden()
    expect(await page.locator('audio').count()).toBeGreaterThanOrEqual(1)
    const roomAudio = page.locator('audio[src*="/api/tracks/"]')
    await expect(async () => {
      expect(await roomAudio.evaluate((el: HTMLAudioElement) => el.paused)).toBe(true)
    }).toPass({ timeout: 10000 })

    // Leaving the room brings the same track back. goBack() is a
    // client-side popstate navigation (no reload), the sidebar drawer is
    // closed inside rooms so it can't be clicked from there.
    await page.goBack()
    await expect(page).toHaveURL('/rooms')
    await expect(
      page.getByRole('button', { name: 'Expand player', exact: true }),
    ).toBeVisible()
    await expect(page.locator('audio').first()).toHaveAttribute('src', src)
    // Rooms pause the shared player on entry; it stays paused, not lost.
    await expect(
      page.getByRole('button', { name: 'Play', exact: true }),
    ).toBeVisible()
  })
})

test.describe('Player transport', () => {
  test('pause/resume, next/prev, queue panel and expanded view', async ({
    page,
  }) => {
    // Two 30 s tone uploads + transcodes under parallel load.
    test.slow()
    const suffix = uniqueId()
    const first = `E2E T1 ${suffix}`
    const second = `E2E T2 ${suffix}`
    await register(page, 'playtrans')
    await uploadTone(page, first, 440)
    await uploadTone(page, second, 660)
    await waitUntilPlayable(page, first)
    await waitUntilPlayable(page, second)

    const src1 = await playTone(page, first)

    // Pause / resume via the bar toggle.
    await page.getByRole('button', { name: 'Pause', exact: true }).click()
    await expect(async () => {
      const paused = await page
        .locator('audio')
        .first()
        .evaluate((el: HTMLAudioElement) => el.paused)
      expect(paused).toBe(true)
    }).toPass({ timeout: 10000 })
    await page.getByRole('button', { name: 'Play', exact: true }).click()
    expect(await audioPlaying(page)).toBe(true)

    // Next / prev switch the audio source.
    await page.getByRole('button', { name: 'Next mashup', exact: true }).click()
    const audio = page.locator('audio').first()
    await expect(async () => {
      expect(await audio.getAttribute('src')).not.toBe(src1)
    }).toPass({ timeout: 15000 })
    const src2 = (await audio.getAttribute('src')) as string
    expect(src2).toMatch(/\/api\/tracks\/.+\/audio/)

    await page.getByRole('button', { name: 'Previous mashup', exact: true }).click()
    await expect(audio).toHaveAttribute('src', src1, { timeout: 15000 })

    // Queue drawer lists what is up next and can start it.
    await page.getByRole('button', { name: 'Queue', exact: true }).click()
    await expect(
      page.getByRole('button', { name: 'Close now playing' }),
    ).toBeVisible()
    await expect(page.getByText(second).first()).toBeVisible()
    await page.getByRole('button', { name: 'Close now playing' }).click()
    await expect(
      page.getByRole('button', { name: 'Close now playing' }),
    ).toBeHidden()

    // Expanded "Now playing" modal opens and collapses.
    await page.getByRole('button', { name: 'Expand player', exact: true }).click()
    await expect(
      page.getByRole('dialog', { name: 'Now playing' }),
    ).toBeVisible()
    await page.getByRole('button', { name: 'Collapse player' }).click()
    await expect(
      page.getByRole('dialog', { name: 'Now playing' }),
    ).toBeHidden()
  })

  test('shuffle/repeat toggles persist and track restores after reload', async ({
    page,
  }) => {
    // 30 s tone upload + transcode, then a full reload and re-ready wait.
    test.slow()
    const title = `E2E Persist ${uniqueId()}`
    await register(page, 'playpersist')
    await uploadTone(page, title)
    await waitUntilPlayable(page, title)
    await playTone(page, title)

    await page.getByRole('button', { name: 'Shuffle', exact: true }).click()
    await expect(
      page.getByRole('button', { name: 'Shuffle', exact: true }),
    ).toHaveAttribute('aria-pressed', 'true')
    await page.getByRole('button', { name: 'Repeat off', exact: true }).click()
    await expect(
      page.getByRole('button', { name: 'Repeat all', exact: true }),
    ).toBeVisible()

    const snapshot = await page.evaluate(
      () => localStorage.getItem('mashup-player'),
    )
    expect(snapshot).toBeTruthy()
    expect(JSON.parse(snapshot as string)).toMatchObject({ shuffle: true })

    // Reload: the walkable list arrives from /mashup, the player restores
    // the same track (offset + toggles), even though autoplay is blocked.
    await page.reload()
    await waitUntilPlayable(page, title)
    await expect(page.locator('audio').first()).toHaveAttribute(
      'src',
      /\/api\/tracks\/.+\/audio/,
      { timeout: 15000 },
    )
    await expect(
      page.getByRole('button', { name: 'Shuffle', exact: true }),
    ).toHaveAttribute('aria-pressed', 'true')
  })
})
