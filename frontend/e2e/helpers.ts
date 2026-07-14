import { Page, expect } from '@playwright/test';

export async function login(page: Page) {
  await page.goto('/login');
  await page.waitForLoadState('networkidle');
  const usernameInput = page.locator('input[id*="username"], input[type="text"]').first();
  const passwordInput = page.locator('input[type="password"]').first();
  await usernameInput.fill('1');
  await passwordInput.fill('1');
  const submitBtn = page.locator('button[type="submit"], button:has-text("登录"), button:has-text("Login")').first();
  await submitBtn.click();
  await page.waitForURL(/^(?!.*login)/, { timeout: 10000 });
}

export async function waitForPageLoad(page: Page) {
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
}

export async function takeScreenshot(page: Page, name: string) {
  const dir = 'e2e/screenshots';
  await page.screenshot({ path: dir + '/' + name + '.png', fullPage: true });
}
