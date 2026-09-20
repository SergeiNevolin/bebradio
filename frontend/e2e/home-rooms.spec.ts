import { expect, test, type Page } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

async function registerUser(page: Page) {
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
}

async function createRoom(page: Page, roomName: string): Promise<string> {
  await page.goto('/rooms')
  await page.getByRole('button', { name: 'Create Room' }).click()
  await page.getByPlaceholder('Room name').fill(roomName)
  await page.getByRole('button', { name: 'Create room', exact: true }).click()
  await expect(page).toHaveURL(/\/room\//)
  return page.url().split('/room/')[1]
}

const roomCard = (page: Page, name: string) =>
  page.getByTestId('room-card').filter({ hasText: name })

/** Cards inside the "All rooms" section (a room can also show up under Recently Played). */
const allRoomsCard = (page: Page, name: string) =>
  page
    .locator('section')
    .filter({ has: page.getByRole('heading', { name: 'All rooms' }) })
    .getByTestId('room-card')
    .filter({ hasText: name })

test('created room appears on the rooms page', async ({ page }) => {
  await registerUser(page)
  const roomName = `Rooms Page ${uniqueId()}`
  await createRoom(page, roomName)

  await page.goto('/rooms')
  await expect(allRoomsCard(page, roomName)).toBeVisible({ timeout: 20000 })
})

test('clicking room card navigates to room', async ({ page }) => {
  await registerUser(page)
  const roomName = `Rooms Nav ${uniqueId()}`
  await createRoom(page, roomName)

  await page.goto('/rooms')
  await allRoomsCard(page, roomName).click()
  await expect(page).toHaveURL(/\/room\//)
  await expect(page.getByRole('heading', { name: roomName })).toBeVisible()
})

test('room card shows room name and code', async ({ page }) => {
  await registerUser(page)
  const roomName = `Rooms Card ${uniqueId()}`
  const roomCode = await createRoom(page, roomName)

  await page.goto('/rooms')
  const card = allRoomsCard(page, roomName)
  await expect(card).toBeVisible({ timeout: 10000 })
  await expect(card.getByTestId('room-card-code')).toHaveText(roomCode)
})

test('live room appears in popular rooms on home page', async ({ page, context }) => {
  await registerUser(page)
  const roomName = `Popular Live ${uniqueId()}`
  const roomCode = await createRoom(page, roomName)

  // A second tab keeps presence so the room counts as live.
  const liveTab = await context.newPage()
  await liveTab.goto(`/room/${roomCode}`)
  await expect(
    liveTab.getByRole('heading', { name: roomName }),
  ).toBeVisible({ timeout: 15000 })

  await page.goto('/')
  await expect(roomCard(page, roomName)).toBeVisible({ timeout: 15000 })
  await liveTab.close()
})

test('rooms page shows room stats', async ({ page }) => {
  await registerUser(page)
  await page.goto('/rooms')
  await expect(page.locator('[class*="roomsHeroStatLabel"]').filter({ hasText: 'rooms' })).toBeVisible()
  await expect(page.locator('[class*="roomsHeroStatLabel"]').filter({ hasText: 'listening' })).toBeVisible()
})

test('navbar links to rooms page', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Комнаты', exact: true }).click()
  await expect(page).toHaveURL('/rooms')
})
