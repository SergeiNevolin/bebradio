import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

test('searches for a track, adds it to queue, and verifies it plays', async ({ page }) => {
  const id = uniqueId()
  const email = `track-${id}@e2e.test`
  const password = 'Test123!'
  const roomName = `Track Test ${id}`

  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(email)
  await page.getByPlaceholder('Username').fill(`track-${id}`)
  await page.getByPlaceholder('Password').fill(password)
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')

  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(roomName)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)

  await page.getByPlaceholder('Search or paste YouTube URL...')
    .fill('Серега Пират прости я не знаю')

  await expect(page.locator('[class*="searchResult"]')).not.toHaveCount(0, { timeout: 30000 })
  await page.locator('[class*="searchResult"]').first().click()

  await expect(page.locator('h3').filter({ hasText: /^Queue \(\d+\)/ })).toBeVisible({ timeout: 120000 })
  expect(await page.locator('[class*="queueItem"]').count()).toBeGreaterThanOrEqual(1)

  const audio = page.locator('audio').first()
  await expect(audio).toHaveAttribute('src', /.+/, { timeout: 120000 })

  const unlockBtn = page.getByRole('button', { name: /Tap to enable sound/ })
  if (await unlockBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
    await unlockBtn.click()
  }

  await expect(async () => {
    const playing = await audio.evaluate((el: HTMLAudioElement) => !el.paused && el.readyState >= 2)
    expect(playing).toBe(true)
  }).toPass({ timeout: 10000 })

  const time1 = await audio.evaluate((el: HTMLAudioElement) => el.currentTime)
  await page.waitForTimeout(1500)
  const time2 = await audio.evaluate((el: HTMLAudioElement) => el.currentTime)

  expect(time2).toBeGreaterThan(time1)
})
