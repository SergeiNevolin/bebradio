import { expect, test } from '@playwright/test'

const uniqueId = () => Math.random().toString(36).slice(2, 8)

const avatarBtn = 'button[aria-label="Open user navigation menu"]'

test.describe('Registration', () => {
  test('registers a new user and lands on home page', async ({ page }) => {
    const id = uniqueId()
    const email = `test-${id}@e2e.test`
    const username = `user-${id}`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Username').fill(username)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()

    await expect(page).toHaveURL('/')
    await expect(page.locator(avatarBtn)).toBeVisible()
  })

  test('shows error on duplicate email', async ({ page }) => {
    const id = uniqueId()
    const email = `dup-${id}@e2e.test`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Username').fill(`dup-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Username').fill(`dup2-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()

    await expect(page.locator('.error-msg').first()).toHaveText('Email already registered')
  })
})

test.describe('Login', () => {
  test('logs in with valid credentials', async ({ page }) => {
    const id = uniqueId()
    const email = `login-${id}@e2e.test`
    const username = `loginuser-${id}`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Username').fill(username)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.evaluate(() => localStorage.clear())
    await page.goto('/login')

    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page).toHaveURL('/')
    await expect(page.locator(avatarBtn)).toBeVisible()
  })

  test('shows error on wrong password', async ({ page }) => {
    await page.goto('/login')
    await page.getByPlaceholder('Email').fill('nonexistent@e2e.test')
    await page.getByPlaceholder('Password').fill('wrong')
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page.locator('.error-msg').first()).toHaveText('Invalid email or password')
  })

  test('navigates from login to register and back', async ({ page }) => {
    await page.goto('/login')
    await page.locator('p').filter({ hasText: "Don't have an account?" }).getByRole('link', { name: 'Register' }).click()
    await expect(page).toHaveURL('/register')

    await page.locator('p').filter({ hasText: 'Already have an account?' }).getByRole('link', { name: 'Sign in' }).click()
    await expect(page).toHaveURL('/login')
  })
})

test.describe('Logout', () => {
  test('logs out and returns to guest state', async ({ page }) => {
    const id = uniqueId()
    const email = `logout-${id}@e2e.test`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Username').fill(`logout-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.locator(avatarBtn).click()
    await page.getByRole('button', { name: 'Sign out' }).click()

    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Register' })).toBeVisible()
  })
})

test.describe('Navbar', () => {
  test('shows guest buttons when not logged in', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Register' })).toBeVisible()
  })

  test('shows user avatar when logged in', async ({ page }) => {
    const id = uniqueId()
    const email = `nav-${id}@e2e.test`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Username').fill(`nav-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await expect(page.locator(avatarBtn)).toBeVisible()
  })
})
