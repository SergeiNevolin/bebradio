import { expect, test } from '@playwright/test'

test.describe('Navigation', () => {
  test('home page shows hero section', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Слушать музыку вместе с друзьями' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Create Room' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Join by Code' })).toBeVisible()
  })

  test('shows 404 for unknown routes', async ({ page }) => {
    await page.goto('/this-page-does-not-exist')
    await expect(page.getByRole('heading', { name: '404' })).toBeVisible()
    await expect(page.getByText('Page not found')).toBeVisible()
    await expect(page.getByRole('link', { name: 'Go Home' })).toBeVisible()
  })

  test('404 page links back to home', async ({ page }) => {
    await page.goto('/nonexistent')
    await page.getByRole('link', { name: 'Go Home' }).click()
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('heading', { name: 'Слушать музыку вместе с друзьями' })).toBeVisible()
  })

  test('login page renders correctly', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('heading', { name: 'bebradio' })).toBeVisible()
    await expect(page.getByPlaceholder('Email')).toBeVisible()
    await expect(page.getByPlaceholder('Password')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible()
    await expect(
      page.locator('p').filter({ hasText: "Don't have an account?" }).getByRole('link', { name: 'Register' }),
    ).toHaveAttribute('href', '/register')
  })

  test('register page renders correctly', async ({ page }) => {
    await page.goto('/register')
    await expect(page.getByRole('heading', { name: 'bebradio' })).toBeVisible()
    await expect(page.getByPlaceholder('Email')).toBeVisible()
    await expect(page.getByPlaceholder('Username')).toBeVisible()
    await expect(page.getByPlaceholder('Password')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Register' })).toBeVisible()
    await expect(
      page.locator('p').filter({ hasText: 'Already have an account?' }).getByRole('link', { name: 'Sign in' }),
    ).toHaveAttribute('href', '/login')
  })

  test('navbar brand links to home', async ({ page }) => {
    await page.goto('/login')
    await page.locator('a[class*="navbarBrand"]').click()
    await expect(page).toHaveURL('/')
  })

  test('mashups page is accessible', async ({ page }) => {
    await page.goto('/mashup')
    await expect(page.locator('.app')).toBeVisible()
  })
})

test.describe('Protected Routes', () => {
  test('settings redirects to login when not authenticated', async ({ page }) => {
    await page.goto('/settings')
    await expect(page).toHaveURL('/login')
  })

  test('settings page accessible when logged in', async ({ page }) => {
    const id = Math.random().toString(36).slice(2, 8)
    const email = `settings-${id}@e2e.test`
    const password = 'Test123!'

    await page.goto('/register')
    await page.getByPlaceholder('Email').fill(email)
    await page.getByPlaceholder('Username').fill(`settings-${id}`)
    await page.getByPlaceholder('Password').fill(password)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page).toHaveURL('/')

    await page.goto('/settings')
    await expect(page).toHaveURL('/settings')
  })
})

test.describe('API Proxy', () => {
  test('proxies API requests to backend', async ({ request }) => {
    const response = await request.get('/api/rooms')
    await expect(response).toBeOK()
  })
})
