import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

test('voting: like, toggle, dislike skips, skip button', async ({ page }) => {
  const id = uniqueId()
  const email = `vote-${id}@e2e.test`
  const password = 'Test123!'

  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(email)
  await page.getByPlaceholder('Username').fill(`vote-${id}`)
  await page.getByPlaceholder('Password').fill(password)
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')

  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(`Vote ${id}`)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)

  await page.getByPlaceholder('Search or paste YouTube URL...')
    .fill('Серега Пират прости я не знаю')
  await expect(page.locator('[class*="searchResult"]')).not.toHaveCount(0, { timeout: 30000 })
  await page.locator('[class*="searchResult"]').first().click()
  await expect(page.locator('h3').filter({ hasText: /^Queue \(\d+\)/ })).toBeVisible({ timeout: 120000 })

  const likeBtn = page.locator('.vote-buttons button').first()
  const dislikeBtn = page.locator('.vote-buttons button').last()
  await expect(likeBtn).toContainText('👍')
  await expect(dislikeBtn).toContainText('👎')

  await likeBtn.click()
  await expect(async () => {
    expect(await likeBtn.textContent()).toMatch(/👍\s*1/)
  }).toPass({ timeout: 5000 })

  await likeBtn.click()
  await expect(async () => {
    expect(await likeBtn.textContent()).toMatch(/👍\s*0/)
  }).toPass({ timeout: 5000 })

  const skipBtn = page.locator('button').filter({ hasText: /Skip/ }).first()
  await expect(skipBtn).toBeVisible()
  await skipBtn.click()

  await page.getByPlaceholder('Search or paste YouTube URL...')
    .fill('Eminem Lose Yourself')
  await expect(page.locator('[class*="searchResult"]')).not.toHaveCount(0, { timeout: 30000 })
  await page.locator('[class*="searchResult"]').first().click()
  await expect(page.locator('h3').filter({ hasText: /^Queue \(\d+\)/ })).toBeVisible({ timeout: 120000 })

  const audio = page.locator('audio').first()
  await expect(audio).toHaveAttribute('src', /.+/, { timeout: 120000 })
  const firstSrc = await audio.getAttribute('src')

  await dislikeBtn.click()
  await expect(async () => {
    const newSrc = await audio.getAttribute('src')
    expect(newSrc).not.toBe(firstSrc)
  }).toPass({ timeout: 10000 })
})
