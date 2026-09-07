import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

async function registerAndCreateRoom(page: import('@playwright/test').Page, roomName: string) {
  const id = uniqueId()
  const email = `settings-${id}@e2e.test`
  const username = `settingsuser-${id}`
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

test('owner can open settings modal', async ({ page }) => {
  const roomName = `Settings Open ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Room Settings' })).toBeVisible()
})

test('toggle private room', async ({ page }) => {
  const roomName = `Settings Private ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Room Settings' })).toBeVisible()

  const toggle = page.locator('.settings-toggle').filter({ hasText: 'Private room' })
  const checkbox = toggle.locator('input[type="checkbox"]')
  await expect(checkbox).toBeChecked({ checked: false })

  await toggle.click()
  await expect(checkbox).toBeChecked()
})

test('toggle auto-radio', async ({ page }) => {
  const roomName = `Settings Radio ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Room Settings' })).toBeVisible()

  const toggle = page.locator('.settings-toggle').filter({ hasText: 'Auto-radio' })
  const checkbox = toggle.locator('input[type="checkbox"]')
  await expect(checkbox).toBeChecked({ checked: false })

  await toggle.click()
  await expect(checkbox).toBeChecked()
})

test('set room password', async ({ page }) => {
  const roomName = `Settings Pass ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Room Settings' })).toBeVisible()

  await page.getByPlaceholder('Set a password').fill('newpass123')
  await page.getByRole('button', { name: 'Set password' }).click()

  await expect(page.getByText('This room is password protected.')).toBeVisible({ timeout: 5000 })
})

test('close settings modal', async ({ page }) => {
  const roomName = `Settings Close ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await page.getByRole('button', { name: 'Settings', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Room Settings' })).toBeVisible()

  await page.getByRole('button', { name: '×' }).click()
  await expect(page.getByRole('heading', { name: 'Room Settings' })).not.toBeVisible()
})
