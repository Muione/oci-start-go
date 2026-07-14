import { test, expect } from '@playwright/test'
import { waitForPageLoad, takeScreenshot } from './helpers'

const SCREENSHOT_PAGES = [
  { path: '/', name: 'dashboard' },
  { path: '/tenants', name: 'tenants' },
  { path: '/instances', name: 'instances' },
  { path: '/storage', name: 'storage' },
  { path: '/vnic', name: 'vnic' },
  { path: '/dns', name: 'dns' },
  { path: '/terminal', name: 'terminal' },
  { path: '/console', name: 'console' },
  { path: '/boot', name: 'boot' },
  { path: '/rescue', name: 'rescue' },
  { path: '/proxies', name: 'proxies' },
  { path: '/migration', name: 'migration' },
  { path: '/settings', name: 'settings' },
]

test.describe('Visual Regression Screenshots', () => {
  test.beforeEach(async ({ page }) => {
    // Set a consistent viewport for screenshots
    await page.setViewportSize({ width: 1440, height: 900 })
  })

  for (const pg of SCREENSHOT_PAGES) {
    test(`screenshot: ${pg.name}`, async ({ page }) => {
      await page.goto(pg.path)

      // Wait for content to load
      await waitForPageLoad(page)

      // Wait for any animations to settle
      await page.waitForTimeout(1000)

      // Take full-page screenshot
      await takeScreenshot(page, `full-${pg.name}`)

      // Also take a viewport screenshot
      await page.screenshot({
        path: `e2e/screenshots/viewport-${pg.name}.png`,
        fullPage: false,
      })
    })
  }

  test('screenshot: login page', async ({ page }) => {
    // Clear auth to see login page
    await page.context().clearCookies()
    await page.goto('/login')

    // Wait for login form to render
    await page.waitForSelector('.ant-card', { timeout: 10_000 })
    await page.waitForTimeout(500)

    await takeScreenshot(page, 'full-login')
  })
})
