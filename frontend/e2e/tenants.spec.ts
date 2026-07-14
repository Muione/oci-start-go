import { test, expect } from '@playwright/test';

test.describe('Tenants (read-only)', () => {
  test('tenant list loads', async ({ page }) => {
    await page.goto('/tenants');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.ant-table-container')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'e2e/screenshots/tenants-list.png', fullPage: true });
  });
});
