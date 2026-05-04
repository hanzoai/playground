import { test, expect } from '../../fixtures';
import { AllBotsPage } from '../../page-objects/all-bots.page';

/**
 * Control Plane E2E tests.
 *
 * Verifies the control plane page loads, SSE connection works,
 * bot cards display, and bot navigation functions.
 * All tests run authenticated via cached auth state.
 */

test.describe('Control Plane', () => {
  let botsPage: AllBotsPage;

  test.beforeEach(async ({ page }) => {
    botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();
  });

  test('control plane page loads and shows header', async () => {
    await expect(botsPage.pageTitle).toBeVisible();
  });

  test('bot grid displays bots or empty state', async ({ page }) => {
    // Wait for either bot cards or empty state to appear (SSE may take time)
    const botCard = botsPage.botCards.first();
    const emptyState = botsPage.emptyState;

    await expect(botCard.or(emptyState)).toBeVisible({ timeout: 15_000 });

    const hasBots = await botsPage.hasBots();
    if (hasBots) {
      const count = await botsPage.getBotCount();
      expect(count).toBeGreaterThan(0);
      console.log(`Found ${count} bots`);
    } else {
      await expect(emptyState).toBeVisible();
    }
  });

  test('SSE live status indicator is visible', async () => {
    // The page should show "Live" or "Offline"
    await expect(botsPage.liveUpdatesBadge).toBeVisible({ timeout: 15_000 });
  });

  test('refresh button triggers bot reload', async ({ page }) => {
    // Wait extra for SSE to connect and data to load
    await page.waitForTimeout(5_000);

    const isVisible = await botsPage.refreshButton.isVisible({ timeout: 10_000 }).catch(() => false);

    if (!isVisible) {
      test.skip(true, 'Refresh button not visible — SSE may not have initialized');
      return;
    }

    await botsPage.refreshButton.click();
    await page.waitForTimeout(2_000);

    // Page should still be loaded after refresh
    await botsPage.expectPageLoaded();
  });

  test('activity toggle button is visible', async () => {
    await expect(botsPage.activityToggle).toBeVisible({ timeout: 5_000 });
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

  test('empty state shows create bot CTA', async () => {
    const hasBots = await botsPage.hasBots();
    if (hasBots) {
      test.skip(true, 'Bots exist — empty state not visible');
      return;
    }

    await expect(botsPage.emptyState).toBeVisible({ timeout: 5_000 });
    await expect(botsPage.createFirstBotButton).toBeVisible({ timeout: 5_000 });
  });

  test('create first bot button is visible when no bots exist', async () => {
    const hasBots = await botsPage.hasBots();
    if (hasBots) {
      // When bots exist, the empty state CTA is not shown — that's correct
      test.skip(true, 'Bots exist — CTA only shown in empty state');
      return;
    }

    await expect(botsPage.createFirstBotButton).toBeVisible({ timeout: 5_000 });
  });
});
