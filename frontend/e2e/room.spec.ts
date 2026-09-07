import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

async function registerAndGoHome(page: import('@playwright/test').Page) {
  const id = uniqueId()
  const email = `room-${id}@e2e.test`
  const username = `roomuser-${id}`
  const password = 'Test123!'

  await page.goto('/register')
  await page.getByPlaceholder('Email').fill(email)
  await page.getByPlaceholder('Username').fill(username)
  await page.getByPlaceholder('Password').fill(password)
  await page.getByRole('button', { name: 'Register' }).click()
  await expect(page).toHaveURL('/')
  return { email, username, password, id }
}

test.describe('Create Room', () => {
  test('creates an open room and navigates to it', async ({ page }) => {
    await registerAndGoHome(page)
    const roomName = `Test Room ${uniqueId()}`

    await page.getByRole('button', { name: 'Create Room' }).click()
    await expect(page.getByRole('heading', { name: 'Create a room' })).toBeVisible()

    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()

    await expect(page).toHaveURL(/\/room\//)
    await expect(page.getByRole('heading', { name: roomName })).toBeVisible()
  })

  test('creates a password-protected room', async ({ page }) => {
    await registerAndGoHome(page)
    const roomName = `Locked Room ${uniqueId()}`

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByPlaceholder('Leave empty for an open room').fill('secret123')
    await page.getByRole('button', { name: 'Create room', exact: true }).click()

    await expect(page).toHaveURL(/\/room\//)
    await expect(page.getByText('🔒')).toBeVisible()
  })

  test('cannot create room without name', async ({ page }) => {
    await registerAndGoHome(page)

    await page.getByRole('button', { name: 'Create Room' }).click()
    const createBtn = page.getByRole('button', { name: 'Create room', exact: true })
    await expect(createBtn).toBeDisabled()
  })

  test('closes create modal on cancel', async ({ page }) => {
    await registerAndGoHome(page)

    await page.getByRole('button', { name: 'Create Room' }).click()
    await expect(page.getByRole('heading', { name: 'Create a room' })).toBeVisible()

    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByRole('heading', { name: 'Create a room' })).not.toBeVisible()
  })
})

test.describe('Join Room', () => {
  test('joins a room by code', async ({ page }) => {
    await registerAndGoHome(page)

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(`Join Test ${uniqueId()}`)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    const code = page.url().split('/room/')[1]
    await page.goto('/')
    await page.getByRole('button', { name: 'Join by Code' }).click()
    await expect(page.getByRole('heading', { name: 'Join a room' })).toBeVisible()

    await page.getByPlaceholder('e.g. ABC123').fill(code)
    await page.getByRole('button', { name: 'Join room' }).click()

    await expect(page).toHaveURL(new RegExp(`/room/${code}`))
  })

  test('shows error for non-existent room code', async ({ page }) => {
    await registerAndGoHome(page)

    await page.getByRole('button', { name: 'Join by Code' }).click()
    await page.getByPlaceholder('e.g. ABC123').fill('ZZZZZZ')
    await page.getByRole('button', { name: 'Join room' }).click()

    await expect(page.locator('.error-msg').first()).toHaveText('Room not found')
  })

  test('closes join modal on cancel', async ({ page }) => {
    await registerAndGoHome(page)

    await page.getByRole('button', { name: 'Join by Code' }).click()
    await expect(page.getByRole('heading', { name: 'Join a room' })).toBeVisible()

    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByRole('heading', { name: 'Join a room' })).not.toBeVisible()
  })
})

test.describe('Password-protected Room', () => {
  test('prompts for password when joining locked room', async ({ page }) => {
    const id = uniqueId()
    const email1 = `owner-${id}@e2e.test`
    const email2 = `guest-${id}@e2e.test`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email1)
    await page.getByPlaceholder('Username').fill(`owner-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(`Locked ${id}`)
    await page.getByPlaceholder('Leave empty for an open room').fill('roompass')
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    const code = page.url().split('/room/')[1]

    await page.evaluate(() => localStorage.clear())
    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email2)
    await page.getByPlaceholder('Username').fill(`guest-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.goto(`/room/${code}`)
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('This room is password protected.')).toBeVisible({ timeout: 10000 })

    await page.getByPlaceholder('Room password').fill('roompass')
    await page.getByRole('button', { name: 'Enter room' }).click()

    await expect(page.getByRole('heading', { name: `Locked ${id}` })).toBeVisible()
  })

  test('rejects wrong password', async ({ page }) => {
    const id = uniqueId()
    const email1 = `owner2-${id}@e2e.test`
    const email2 = `guest2-${id}@e2e.test`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email1)
    await page.getByPlaceholder('Username').fill(`owner2-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(`Wrong Pass ${id}`)
    await page.getByPlaceholder('Leave empty for an open room').fill('correct')
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)
    const code = page.url().split('/room/')[1]

    await page.evaluate(() => localStorage.clear())
    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email2)
    await page.getByPlaceholder('Username').fill(`guest2-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.goto(`/room/${code}`)
    await page.waitForLoadState('networkidle')
    await page.getByPlaceholder('Room password').fill('wrong')
    await page.getByRole('button', { name: 'Enter room' }).click()

    await expect(page.locator('.error-msg').first()).toHaveText('Incorrect room password')
  })
})

test.describe('Room Page', () => {
  test('shows room header with name and listeners count', async ({ page }) => {
    await registerAndGoHome(page)
    const roomName = `Header Test ${uniqueId()}`

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    await expect(page.getByRole('heading', { name: roomName })).toBeVisible()
    await expect(page.locator('[class*="roomStatus"]')).toBeVisible()
  })

  test('owner sees settings button', async ({ page }) => {
    await registerAndGoHome(page)
    const roomName = `Settings Test ${uniqueId()}`

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    await expect(page.getByRole('button', { name: 'Settings' })).toBeVisible()
  })

  test('room page shows add track input for open rooms', async ({ page }) => {
    await registerAndGoHome(page)
    const roomName = `Track Input ${uniqueId()}`

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    await expect(page.getByPlaceholder('Search or paste YouTube URL...')).toBeVisible()
  })

  test('share button copies room URL', async ({ page }) => {
    await registerAndGoHome(page)
    const roomName = `Share Test ${uniqueId()}`

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    await expect(page.getByRole('button', { name: 'Share' })).toBeVisible()
  })
})

test.describe('Room Settings', () => {
  test('owner can toggle anonymous add', async ({ page }) => {
    await registerAndGoHome(page)
    const roomName = `Toggle Test ${uniqueId()}`

    await page.getByRole('button', { name: 'Create Room' }).click()
    await page.getByPlaceholder('Room name').fill(roomName)
    await page.getByRole('button', { name: 'Create room', exact: true }).click()
    await expect(page).toHaveURL(/\/room\//)

    await page.getByRole('button', { name: 'Settings' }).click()
    await expect(page.getByRole('heading', { name: 'Room Settings' })).toBeVisible()

    const toggleLabel = page.locator('label.settings-toggle', { hasText: 'Allow anonymous' })
    const toggleInput = toggleLabel.locator('input[type="checkbox"]')

    await toggleLabel.click()
    await expect(toggleInput).not.toBeChecked()

    await toggleLabel.click()
    await expect(toggleInput).toBeChecked()
  })
})
