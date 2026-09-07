import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

async function registerAndCreateRoom(page: import('@playwright/test').Page, roomName: string) {
  const id = uniqueId()
  const email = `queue-${id}@e2e.test`
  const username = `queueuser-${id}`
  const password = 'Test123!'

  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(email)
  await page.getByPlaceholder('Username').fill(username)
  await page.getByPlaceholder('Password').fill(password)
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')

  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(roomName)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)
  return { email, username, password, id }
}

async function addTrack(page: import('@playwright/test').Page, query: string) {
  await page.getByPlaceholder('Search or paste YouTube URL...').fill(query)
  await expect(page.locator('[class*="searchResult"]')).not.toHaveCount(0, { timeout: 30000 })
  await page.locator('[class*="searchResult"]').first().click()
}

const queueHasCount = (page: import('@playwright/test').Page) =>
  page.locator('h3').filter({ hasText: /^Queue \(\d+\)/ })

test('queue shows tracks after adding', async ({ page }) => {
  const roomName = `Queue Show ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await addTrack(page, 'Серега Пират прости я не знаю')
  await expect(queueHasCount(page)).toBeVisible({ timeout: 120000 })

  const queueCount = await page.locator('[class*="queueItem"]').count()
  expect(queueCount).toBeGreaterThanOrEqual(1)
})

test('adds second track to queue', async ({ page }) => {
  const roomName = `Queue Two ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await addTrack(page, 'Серега Пират прости я не знаю')
  await expect(queueHasCount(page)).toBeVisible({ timeout: 120000 })

  await addTrack(page, 'Eminem Lose Yourself')
  await expect(page.locator('h3').filter({ hasText: 'Queue (2)' })).toBeVisible({ timeout: 120000 })
})

test('skip advances to next track', async ({ page }) => {
  const roomName = `Queue Skip ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await addTrack(page, 'Серега Пират прости я не знаю')
  await expect(queueHasCount(page)).toBeVisible({ timeout: 120000 })

  await addTrack(page, 'Eminem Lose Yourself')
  await expect(page.locator('h3').filter({ hasText: 'Queue (2)' })).toBeVisible({ timeout: 120000 })

  const audio = page.locator('audio').first()
  await expect(audio).toHaveAttribute('src', /.+/, { timeout: 120000 })

  await page.locator('button').filter({ hasText: /Skip/ }).first().click()

  await expect(async () => {
    const playing = await audio.evaluate((el: HTMLAudioElement) => !el.paused && el.readyState >= 2)
    expect(playing).toBe(true)
  }).toPass({ timeout: 15000 })
})

test('queue heading updates count', async ({ page }) => {
  const roomName = `Queue Count ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await expect(page.locator('h3').filter({ hasText: 'Queue' })).toBeVisible()

  await addTrack(page, 'Серега Пират прости я не знаю')
  await expect(page.locator('h3').filter({ hasText: 'Queue (1)' })).toBeVisible({ timeout: 120000 })
})
