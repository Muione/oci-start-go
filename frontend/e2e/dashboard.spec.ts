import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
  test('loads with stat cards', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);
    await expect(page.locator('.ant-card').first()).toBeVisible({ timeout: 10000 });
  });
});
