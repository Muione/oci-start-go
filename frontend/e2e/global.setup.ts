import { test as setup } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const authDir = path.join(__dirname, '.auth');
const authFile = path.join(authDir, 'user.json');

setup('authenticate', async ({ page }) => {
  fs.mkdirSync(authDir, { recursive: true });

  await page.goto('/login');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(2000);

  // Ant Design inputs - use .ant-input selector
  const inputs = page.locator('.ant-input');
  const usernameInput = inputs.nth(0);
  const passwordInput = page.locator('.ant-input-password .ant-input, input[type="password"]').first();

  await usernameInput.click();
  await usernameInput.fill('1');
  await passwordInput.click();
  await passwordInput.fill('1');

  // Ant Design submit button
  const submitBtn = page.locator('button[type="submit"]').first();
  await submitBtn.click();

  // Wait for navigation or check if we're still on login
  try {
    await page.waitForURL(/^(?!.*login)/, { timeout: 15000 });
  } catch {
    // If still on login, take screenshot for debugging
    await page.screenshot({ path: 'e2e/screenshots/debug-login-failed.png', fullPage: true });
    // Try pressing Enter as fallback
    await passwordInput.press('Enter');
    await page.waitForURL(/^(?!.*login)/, { timeout: 10000 });
  }

  await page.context().storageState({ path: authFile });
});
