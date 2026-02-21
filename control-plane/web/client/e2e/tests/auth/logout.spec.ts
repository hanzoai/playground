import { test, expect } from '@playwright/test';
import { TopNavigationPage } from '../../page-objects/top-navigation.page';

/**
 * Logout flow E2E test.
 *
 * Verifies that signing out clears tokens and returns to auth guard.
 */

test.describe('Logout Flow', () => {
  test('sign out clears auth and shows login screen', async ({ page }) => {
    const nav = new TopNavigationPage(page);

    await page.goto('/dashboard', { waitUntil: 'networkidle' });

    // Verify we're authenticated
    await expect(page).toHaveURL(/\/dashboard/);

    // Sign out via user menu
    await nav.signOut();

    // Should see the auth guard again
    const signInButton = page.getByRole('button', { name: /sign in with hanzo/i });
    await expect(signInButton).toBeVisible({ timeout: 15_000 });

    // Token should be cleared from localStorage
    const token = await page.evaluate(() => localStorage.getItem('af_iam_token'));
    expect(token).toBeFalsy();
  });

  test('after logout, protected routes redirect to login', async ({ page }) => {
    const nav = new TopNavigationPage(page);

    await page.goto('/dashboard', { waitUntil: 'networkidle' });
    await nav.signOut();

    // Wait for auth guard
    await expect(page.getByRole('button', { name: /sign in with hanzo/i })).toBeVisible({ timeout: 15_000 });

    // Try navigating to a protected route
    await page.goto('/bots/all', { waitUntil: 'networkidle' });

    // Should see auth guard, not bots page
    const authGuard = page.getByRole('button', { name: /sign in with hanzo/i });
    await expect(authGuard).toBeVisible({ timeout: 10_000 });
  });
});
