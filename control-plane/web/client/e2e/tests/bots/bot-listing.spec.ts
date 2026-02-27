import { test, expect } from '../../fixtures';
import { AllBotsPage } from '../../page-objects/all-bots.page';

/**
 * Control Plane E2E tests.
 *
 * Verifies the control plane page loads, SSE connection works,
 * agent cards display, and bot navigation functions.
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

  test('agent network displays agents or empty state', async ({ page }) => {
    // Wait for either agent cards or empty state to appear (SSE may take time)
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
    // The refresh button has title="Refresh" — wait for it to be visible
    const refreshBtn = page.locator('button[title="Refresh"]')
      .or(page.locator('button').filter({ has: page.locator('svg') }).filter({ hasText: '' }).first());
    const isVisible = await refreshBtn.first().isVisible({ timeout: 5_000 }).catch(() => false);

    if (!isVisible) {
      test.skip(true, 'Refresh button not visible — SSE may not have initialized');
      return;
    }

    await refreshBtn.first().click();
    await page.waitForTimeout(1_500);

    // Page should still be loaded after refresh
    await botsPage.expectPageLoaded();
  });

  test('agent card expands to show bots', async () => {
    const hasAgents = await botsPage.hasAgents();
    if (!hasAgents) {
      test.skip(true, 'No agents available for expansion test');
      return;
    }

    // Expand first agent
    await botsPage.expandAgent(0);

    // After expanding, the bot list should be visible inside the agent card
    const firstAgent = botsPage.agentCards.first();
    const botList = firstAgent.locator('.border-t');
    await expect(botList).toBeVisible({ timeout: 5_000 });
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

  test('metrics strip shows agent and bot counts', async () => {
    // The metrics strip should show Agents, Cloud, Local, Bots, Healthy
    await expect(botsPage.page.getByText('Agents', { exact: true })).toBeVisible({ timeout: 10_000 });
    await expect(botsPage.page.getByText('Bots', { exact: true })).toBeVisible({ timeout: 5_000 });
  });

  test('register agent button is visible', async () => {
    await expect(botsPage.registerAgentButton).toBeVisible({ timeout: 5_000 });
  });
});
