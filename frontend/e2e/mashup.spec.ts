import { expect, test, type Page } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

/**
 * Builds a small PCM16 mono WAV in memory so the suite needs no binary
 * fixtures committed to the repo. A couple of seconds is plenty: the
 * music-service transcodes it to m4a in well under a minute.
 */
function makeToneWav(seconds = 2, freq = 440): Buffer {
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

/** Uploads a tone WAV through the UI and returns its unique title. */
async function uploadTone(page: Page, title: string) {
  await page.goto('/mashup')
  await page.getByRole('button', { name: 'Upload', exact: true }).click()

  const modal = page.locator('div.modal')
  await modal.locator('#mashup-file').setInputFiles({
    name: 'tone.wav',
    mimeType: 'audio/wav',
    buffer: makeToneWav(),
  })
  await modal.getByPlaceholder('Track title').fill(title)
  await modal.getByRole('button', { name: 'Upload', exact: true }).click()

  // The modal closes on success; the card lands in Latest / My mashups.
  await expect(modal).toBeHidden({ timeout: 120000 })
  await expect(page.getByText(title).first()).toBeVisible()
}

/** Waits until the uploaded card flips from processing to playable. */
async function waitUntilPlayable(page: Page, title: string) {
  // The fresh upload lands in both Latest and My mashups shelves.
  await expect(
    page.getByRole('button', { name: `Play ${title}`, exact: true }).first(),
  ).toBeVisible({ timeout: 180000 })
}

test.describe('Mashup upload', () => {
  test('uploads a mashup and sees it become ready', async ({ page }) => {
    const title = `E2E Tone ${uniqueId()}`
    await register(page, 'mashup')
    await uploadTone(page, title)
    await waitUntilPlayable(page, title)
  })
})

test.describe('Mashup playback', () => {
  test('plays an uploaded mashup end to end', async ({ page }) => {
    const title = `E2E Play ${uniqueId()}`
    await register(page, 'mashplay')
    await uploadTone(page, title)
    await waitUntilPlayable(page, title)

    await page
      .getByRole('button', { name: `Play ${title}`, exact: true })
      .first()
      .click()

    const audio = page.locator('audio').first()
    await expect(audio).toHaveAttribute('src', /\/api\/tracks\/.+\/audio/, {
      timeout: 15000,
    })

    // Trusted click counts as a user gesture, so play() is allowed.
    await expect(async () => {
      const state = await audio.evaluate((el: HTMLAudioElement) => ({
        paused: el.paused,
        readyState: el.readyState,
      }))
      expect(state.paused).toBe(false)
      expect(state.readyState).toBeGreaterThanOrEqual(2)
    }).toPass({ timeout: 30000 })

    const time1 = await audio.evaluate((el: HTMLAudioElement) => el.currentTime)
    await page.waitForTimeout(1500)
    const time2 = await audio.evaluate((el: HTMLAudioElement) => el.currentTime)
    expect(time2).toBeGreaterThan(time1)
  })
})
