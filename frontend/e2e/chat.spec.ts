import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

async function registerAndCreateRoom(page: import('@playwright/test').Page, roomName: string) {
  const id = uniqueId()
  const email = `chat-${id}@e2e.test`
  const username = `chatuser-${id}`
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

test('sends a chat message and it appears in the chat', async ({ page }) => {
  const roomName = `Chat Test ${uniqueId()}`
  const { username } = await registerAndCreateRoom(page, roomName)
  const message = `Hello from e2e ${uniqueId()}`

  await page.getByPlaceholder('Type a message...').fill(message)
  await page.getByRole('button', { name: 'Send' }).click()

  await expect(page.getByText(message)).toBeVisible({ timeout: 10000 })
  await expect(page.locator('[class*="chatMessageOwn"]').filter({ hasText: username })).toBeVisible()
})

test('chat input clears after sending', async ({ page }) => {
  const roomName = `Chat Clear ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  const input = page.getByPlaceholder('Type a message...')
  await input.fill('test message')
  await page.getByRole('button', { name: 'Send' }).click()

  await expect(input).toHaveValue('')
})

test('send button is disabled when input is empty', async ({ page }) => {
  const roomName = `Chat Disabled ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await expect(page.getByRole('button', { name: 'Send' })).toBeDisabled()
})

test('shows empty state when no messages', async ({ page }) => {
  const roomName = `Chat Empty ${uniqueId()}`
  await registerAndCreateRoom(page, roomName)

  await expect(page.getByText('No messages yet')).toBeVisible()
})
