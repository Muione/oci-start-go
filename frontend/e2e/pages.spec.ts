import { test, expect } from '@playwright/test';
import * as fs from 'fs';

const pages = [
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
];

test.describe('Page Smoke Tests + Screenshots', () => {
  fs.mkdirSync('e2e/screenshots', { recursive: true });

  for (const p of pages) {
    test(`${p.name} page loads`, async ({ page }) => {
      const errors: string[] = [];
      page.on('pageerror', (err) => errors.push(err.message));

      await page.goto(p.path);
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(1000);

      await page.screenshot({
        path: `e2e/screenshots/${p.name}.png`,
        fullPage: true,
      });

      console.log(`Screenshot: e2e/screenshots/${p.name}.png`);
      expect(errors.filter(e => !e.includes('ResizeObserver')).length).toBe(0);
    });
  }
});
