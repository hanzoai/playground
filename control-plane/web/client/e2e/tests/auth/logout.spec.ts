import { test, expect } from '../../fixtures';
import { TopNavigationPage } from '../../page-objects/top-navigation.page';

/**
 * Logout flow E2E test.
 *
 * Verifies that signing out clears tokens and redirects to IAM login.
 * After sign-out the app redirects to hanzo.id (IAM) — NOT back to an
 * in-app auth guard. We verify that we leave the authenticated app URL.
 */

test.describe('Logout Flow', () => {
  test('sign out clears auth and shows login screen', async ({ page }) => {
    const nav = new TopNavigationPage(page);

    await page.goto('/launch', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle').catch(() => {});

    // Verify we're authenticated — on an app page
    await expect(page).toHaveURL(/\/(launch|dashboard|bots|nodes)/);

    // Sign out via user menu
    await nav.signOut();

    // After logout the app either:
    //  a) redirects to hanzo.id login page, OR
    //  b) shows "Redirecting to sign in..." auth guard text
    const iamPage = page.waitForURL(/hanzo\.id/, { timeout: 20_000 }).then(() => 'iam' as const);
    const authGuard = page.getByText(/redirecting to sign in|sign in/i)
      .waitFor({ state: 'visible', timeout: 20_000 })
      .then(() => 'guard' as const);

    const landed = await Promise.race([iamPage, authGuard]);
    expect(['iam', 'guard']).toContain(landed);

    // Token should be cleared from localStorage (evaluate on current page)
    const token = await page.evaluate(() => localStorage.getItem('af_iam_token'));
    expect(token).toBeFalsy();
  });

  test('after logout, session tokens are cleared', async ({ page }) => {
    const nav = new TopNavigationPage(page);

    await page.goto('/launch', { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle').catch(() => {});
    await nav.signOut();

    // Wait for redirect away from app
    const iamPage = page.waitForURL(/hanzo\.id/, { timeout: 20_000 }).then(() => 'iam' as const);
    const authGuard = page.getByText(/redirecting to sign in|sign in/i)
      .waitFor({ state: 'visible', timeout: 20_000 })
      .then(() => 'guard' as const);
    await Promise.race([iamPage, authGuard]);

    // If we redirected to hanzo.id, go back to the app to check storage is cleared
    if (page.url().includes('hanzo.id')) {
      const baseURL = process.env.E2E_BASE_URL || 'https://app.hanzo.bot';
      await page.goto(baseURL, { waitUntil: 'domcontentloaded' });
    }

    // Verify IAM session tokens are cleared
    const sessionToken = await page.evaluate(() => sessionStorage.getItem('hanzo_iam_access_token'));
    expect(sessionToken).toBeFalsy();

    // Verify localStorage auth is cleared
    const localToken = await page.evaluate(() => localStorage.getItem('af_iam_token'));
    expect(localToken).toBeFalsy();
  });
});
