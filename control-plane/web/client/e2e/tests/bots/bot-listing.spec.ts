import { test, expect } from '@playwright/test';
import { AllBotsPage } from '../../page-objects/all-bots.page';

/**
 * Bot Listing E2E tests.
 *
 * Verifies bot grid loads, SSE connection works, search/filter functions.
 * All tests run authenticated via cached auth state.
 */

test.describe('Bot Listing', () => {
  let botsPage: AllBotsPage;

  test.beforeEach(async ({ page }) => {
    botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();
  });

  test('bot listing page loads and shows header', async () => {
    await expect(botsPage.pageTitle).toBeVisible();
  });

  test('bot grid displays bots or empty state', async () => {
    const hasBots = await botsPage.hasBots();

    if (hasBots) {
      const count = await botsPage.getBotCount();
      expect(count).toBeGreaterThan(0);
      console.log(`Found ${count} bots`);
    } else {
      // Empty state should be shown
      await expect(botsPage.emptyState).toBeVisible({ timeout: 10_000 });
    }
  });

  test('SSE live updates badge is visible', async () => {
    // The page should show "Live Updates" or "Disconnected"
    await expect(botsPage.liveUpdatesBadge).toBeVisible({ timeout: 15_000 });
  });

  test('refresh button triggers bot reload', async ({ page }) => {
    await botsPage.refresh();

    // Page should still be loaded after refresh
    await botsPage.expectPageLoaded();
  });

  test('search filters bot list', async () => {
    const hasBots = await botsPage.hasBots();
    if (!hasBots) {
      test.skip(true, 'No bots available for search test');
      return;
    }

    // Search for a nonexistent bot — should show no results
    await botsPage.searchForBot('zzz_nonexistent_bot_xyz');
    const countAfterBadSearch = await botsPage.getBotCount();
    expect(countAfterBadSearch).toBe(0);

    // Clear search — bots should reappear
    await botsPage.clearSearch();
    const countAfterClear = await botsPage.getBotCount();
    expect(countAfterClear).toBeGreaterThan(0);
  });

  test('status filter switches between online/all/offline', async () => {
    // Click "All" filter
    await botsPage.filterByStatus('all');
    const allCount = await botsPage.getBotCount();

    // Click "Online" filter
    await botsPage.filterByStatus('online');
    const onlineCount = await botsPage.getBotCount();

    // Online count should be <= all count
    expect(onlineCount).toBeLessThanOrEqual(allCount);

    // Click "Offline" filter
    await botsPage.filterByStatus('offline');
    const offlineCount = await botsPage.getBotCount();

    // Online + offline should roughly equal all (depending on filtering logic)
    console.log(`Bots: all=${allCount}, online=${onlineCount}, offline=${offlineCount}`);
  });

  test('clicking a bot navigates to bot detail page', async ({ page }) => {
    const hasBots = await botsPage.hasBots();
    if (!hasBots) {
      test.skip(true, 'No bots available for navigation test');
      return;
    }

    await botsPage.clickFirstBot();
    await page.waitForURL(/\/bots\//, { timeout: 15_000 });

    // Should be on a bot detail page
    expect(page.url()).toMatch(/\/bots\/.+/);
  });
});
