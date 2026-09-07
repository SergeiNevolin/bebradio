import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

async function registerUser(page: import('@playwright/test').Page) {
  const id = uniqueId()
  const email = `home-${id}@e2e.test`
  const username = `homeuser-${id}`
  const password = 'Test123!'

  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(email)
  await page.getByPlaceholder('Username').fill(username)
  await page.getByPlaceholder('Password').fill(password)
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')
  return { email, username, password, id }
}

test('created room appears in Top Rooms on home page', async ({ page }) => {
  await registerUser(page)
  const roomName = `Home Room ${uniqueId()}`

  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(roomName)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)

  await page.goto('/')
  await expect(page.locator('[class*="homeCardName"]').filter({ hasText: roomName }).first()).toBeVisible({ timeout: 10000 })
})

test('clicking room card navigates to room', async ({ page }) => {
  await registerUser(page)
  const roomName = `Home Nav ${uniqueId()}`

  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(roomName)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)

  await page.goto('/')
  await expect(page.locator('[class*="homeCardName"]').filter({ hasText: roomName }).first()).toBeVisible({ timeout: 10000 })

  await page.locator('[class*="homeCardName"]').filter({ hasText: roomName }).first().click()
  await expect(page).toHaveURL(/\/room\//)
  await expect(page.getByRole('heading', { name: roomName })).toBeVisible()
})

test('room card shows room name and code', async ({ page }) => {
  await registerUser(page)
  const roomName = `Home Card ${uniqueId()}`

  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(roomName)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)

  const roomCode = page.url().split('/room/')[1]

  await page.goto('/')
  const card = page.locator('[class*="homeCardName"]').filter({ hasText: roomName }).first()
  await expect(card).toBeVisible({ timeout: 10000 })

  const parentCard = card.locator('..').locator('..')
  await expect(parentCard.locator('[class*="homeCardId"]').filter({ hasText: roomCode })).toBeVisible()
})

test('home page shows room stats', async ({ page }) => {
  await registerUser(page)
  await expect(page.locator('[class*="homeHeroStatLabel"]').filter({ hasText: 'rooms' })).toBeVisible()
  await expect(page.locator('[class*="homeHeroStatLabel"]').filter({ hasText: 'listening' })).toBeVisible()
})
