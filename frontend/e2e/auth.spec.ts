import { test, expect } from '@playwright/test';

test.describe('Auth', () => {
  test('dashboard loads after login', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.ant-card').first()).toBeVisible({ timeout: 10000 });
  });
});
