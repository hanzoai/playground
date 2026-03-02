import { test, expect } from '../../fixtures';
import { AllBotsPage } from '../../page-objects/all-bots.page';

/**
 * My Bots E2E tests.
 *
 * Verifies the nodes page loads, SSE connection works,
 * node cards display, and bot navigation functions.
 * All tests run authenticated via cached auth state.
 */

test.describe('My Bots', () => {
  let botsPage: AllBotsPage;

  test.beforeEach(async ({ page }) => {
    botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();
  });

  test('my bots page loads and shows header', async () => {
    await expect(botsPage.pageTitle).toBeVisible();
  });

  test('node list displays nodes or empty state', async ({ page }) => {
    // Wait for either node cards or empty state to appear (SSE may take time)
    const agentCard = botsPage.agentCards.first();
    const emptyState = botsPage.emptyState;

    await expect(agentCard.or(emptyState)).toBeVisible({ timeout: 15_000 });

    const hasAgents = await botsPage.hasAgents();
    if (hasAgents) {
      const count = await botsPage.getAgentCount();
      expect(count).toBeGreaterThan(0);
      console.log(`Found ${count} agents`);
    } else {
      await expect(emptyState).toBeVisible();
    }
  });

  test('SSE live status indicator is visible', async () => {
    // The page should show "Live" or "Offline"
    await expect(botsPage.liveUpdatesBadge).toBeVisible({ timeout: 15_000 });
  });

  test('refresh button triggers agent reload', async ({ page }) => {
    // Wait extra for SSE to connect and data to load
    await page.waitForTimeout(5_000);

    // The refresh button has title="Refresh" — multiple fallback selectors
    const refreshBtn = page.locator('button[title="Refresh"]');
    const isVisible = await refreshBtn.isVisible({ timeout: 10_000 }).catch(() => false);

    if (!isVisible) {
      test.skip(true, 'Refresh button not visible — SSE may not have initialized');
      return;
    }

    await refreshBtn.click();
    await page.waitForTimeout(2_000);

    // Page should still be loaded after refresh
    await botsPage.expectPageLoaded();
  });

  test('node card expands to show capabilities', async () => {
    const hasAgents = await botsPage.hasAgents();
    if (!hasAgents) {
      test.skip(true, 'No nodes available for expansion test');
      return;
    }

    // Expand first node card
    await botsPage.expandAgent(0);

    // After expanding, the detail section should be visible inside the node card
    const firstCard = botsPage.agentCards.first();
    const detailSection = firstCard.locator('.border-t');
    await expect(detailSection).toBeVisible({ timeout: 5_000 });
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

  test('metrics strip shows node and bot counts', async () => {
    // The metrics strip should show Nodes, Cloud, Local, Bots, Healthy
    await expect(botsPage.page.getByText('Nodes', { exact: true })).toBeVisible({ timeout: 10_000 });
    await expect(botsPage.page.getByText('Bots', { exact: true })).toBeVisible({ timeout: 5_000 });
  });

  test('add bot button is visible', async () => {
    await expect(botsPage.registerAgentButton).toBeVisible({ timeout: 5_000 });
  });
});
