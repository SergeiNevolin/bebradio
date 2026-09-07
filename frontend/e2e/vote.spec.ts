import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

async function registerAndCreateRoom(page: import('@playwright/test').Page, roomName: string) {
  const id = uniqueId()
  const email = `vote-${id}@e2e.test`
  const username = `voteuser-${id}`
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
  await expect(page.locator('h3').filter({ hasText: /^Queue \(\d+\)/ })).toBeVisible({ timeout: 120000 })
  const audio = page.locator('audio').first()
  await expect(audio).toHaveAttribute('src', /.+/, { timeout: 120000 })
}

test('like button increments like count', async ({ page }) => {
  const roomName = `Vote Like ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)
  await addTrack(page, 'Серега Пират прости я не знаю')

  const likeBtn = page.locator('.vote-buttons button').first()
  await expect(likeBtn).toContainText('👍')
  await likeBtn.click()

  await expect(async () => {
    const text = await likeBtn.textContent()
    expect(text).toMatch(/👍\s*1/)
  }).toPass({ timeout: 5000 })
})

test('dislike auto-skips when dislikes exceed likes', async ({ page }) => {
  const roomName = `Vote Dislike ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await addTrack(page, 'Серега Пират прости я не знаю')
  await addTrack(page, 'Eminem Lose Yourself')
  await expect(page.locator('h3').filter({ hasText: 'Queue (2)' })).toBeVisible({ timeout: 120000 })

  const audio = page.locator('audio').first()
  const firstSrc = await audio.getAttribute('src')

  const dislikeBtn = page.locator('.vote-buttons button').last()
  await expect(dislikeBtn).toContainText('👎')
  await dislikeBtn.click()

  await expect(async () => {
    const newSrc = await audio.getAttribute('src')
    expect(newSrc).not.toBe(firstSrc)
  }).toPass({ timeout: 10000 })
})

test('like toggles off on second click', async ({ page }) => {
  const roomName = `Vote Toggle ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)
  await addTrack(page, 'Серега Пират прости я не знаю')

  const likeBtn = page.locator('.vote-buttons button').first()
  await expect(likeBtn).toContainText('👍')
  await likeBtn.click()
  await expect(async () => {
    const text = await likeBtn.textContent()
    expect(text).toMatch(/👍\s*1/)
  }).toPass({ timeout: 5000 })

  await likeBtn.click()
  await expect(async () => {
    const text = await likeBtn.textContent()
    expect(text).toMatch(/👍\s*0/)
  }).toPass({ timeout: 5000 })
})

test('skip vote button is visible and clickable', async ({ page }) => {
  const roomName = `Vote Skip ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)
  await addTrack(page, 'Серега Пират прости я не знаю')

  const skipBtn = page.locator('button').filter({ hasText: /Skip/ }).first()
  await expect(skipBtn).toBeVisible()
  await skipBtn.click()
})
