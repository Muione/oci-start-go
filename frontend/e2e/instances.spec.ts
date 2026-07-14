import { test, expect } from '@playwright/test';

test.describe('Instances (read-only)', () => {
  test('instance list loads', async ({ page }) => {
    await page.goto('/instances');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.ant-table-container')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'e2e/screenshots/instances-list.png', fullPage: true });
  });
});
