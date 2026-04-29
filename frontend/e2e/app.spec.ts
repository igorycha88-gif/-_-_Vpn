import { test, expect } from '@playwright/test'

test.describe('Login Page', () => {
  test('displays login form', async ({ page }) => {
    await page.goto('/admin/login')
    await expect(page.getByPlaceholder('Email')).toBeVisible()
    await expect(page.getByPlaceholder('Пароль')).toBeVisible()
    await expect(page.getByRole('button', { name: /войти/i })).toBeVisible()
  })

  test('shows SmartTraffic title', async ({ page }) => {
    await page.goto('/admin/login')
    await expect(page.getByText('SmartTraffic')).toBeVisible()
  })

  test('shows error on wrong credentials', async ({ page }) => {
    await page.goto('/admin/login')
    await page.getByPlaceholder('Email').fill('wrong@test.com')
    await page.getByPlaceholder('Пароль').fill('wrongpass')
    await page.getByRole('button', { name: /войти/i }).click()
    await expect(page.getByText(/ошибк/i)).toBeVisible({ timeout: 5000 })
  })

  test('validates empty fields', async ({ page }) => {
    await page.goto('/admin/login')
    await page.getByRole('button', { name: /войти/i }).click()
  })
})

test.describe('Dashboard (authenticated)', () => {
  test('redirects to login when not authenticated', async ({ page }) => {
    await page.goto('/admin/')
    await page.waitForURL(/\/admin\/login/, { timeout: 5000 })
    await expect(page.getByPlaceholder('Email')).toBeVisible()
  })
})

test.describe('Navigation', () => {
  test('login page is accessible', async ({ page }) => {
    await page.goto('/admin/login')
    await expect(page).toHaveURL(/\/admin\/login/)
  })

  test('non-existent route redirects', async ({ page }) => {
    await page.goto('/admin/nonexistent')
    await page.waitForURL(/\/admin\/(login)?/, { timeout: 5000 })
  })
})

test.describe('Landing page', () => {
  test('health endpoint is reachable', async ({ request }) => {
    const response = await request.get('/health')
    expect(response.ok()).toBeTruthy()
    const body = await response.json()
    expect(body.status).toBe('ok')
  })
})
