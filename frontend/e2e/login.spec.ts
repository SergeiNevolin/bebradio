import { expect, test } from '@playwright/test'

test('login page allows entering credentials', async ({ page }) => {
  await page.goto('/login')

  await expect(page.getByRole('heading', { name: 'bebradio' })).toBeVisible()
  await expect(page.getByPlaceholder('Email')).toBeVisible()
  await expect(page.getByPlaceholder('Password')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible()
  await expect(
    page.locator('p').filter({ hasText: "Don't have an account?" }).getByRole('link', { name: 'Register' }),
  ).toHaveAttribute('href', '/register')
})

test('frontend proxies requests to the backend', async ({ request }) => {
  const response = await request.get('/api/rooms')

  await expect(response).toBeOK()
})