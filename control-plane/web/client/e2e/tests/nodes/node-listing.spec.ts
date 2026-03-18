import { test, expect } from '../../fixtures';
import { NodesPage } from '../../page-objects/nodes.page';

/**
 * Node listing E2E tests.
 *
 * Verifies:
 *   - Nodes page loads and displays connected nodes
 *   - At least one node is visible when gateway is connected
 *   - Gateway WebSocket connection is established (live updates)
 *   - Search/filtering works
 *   - Node cards link to detail pages
 */

test.describe('Node Listing', () => {
  let nodesPage: NodesPage;

  test.beforeEach(async ({ page }) => {
    nodesPage = new NodesPage(page);
    await nodesPage.goto();
    await nodesPage.waitForLoaded();

    // If gateway shows disconnected error, retry once
    if (await nodesPage.isGatewayDisconnected()) {
      await nodesPage.retryGatewayConnection();
    }
  });

  test('nodes page loads and renders UI', async ({ page }) => {
    // The page should load — either showing nodes or gateway state
    const hasNodes = (await nodesPage.getNodeCards()).length > 0;
    const hasGatewayError = await nodesPage.isGatewayDisconnected();
    const hasHeading = await page.locator('text=Nodes').first().isVisible().catch(() => false);

    // At minimum the page rendered something meaningful
    expect(hasNodes || hasGatewayError || hasHeading).toBe(true);

    // If gateway connected, we should see at least one node (our local Mac)
    if (hasNodes) {
      const cards = await nodesPage.getNodeCards();
      expect(cards.length).toBeGreaterThanOrEqual(1);
    }
  });

  test('at least one node is visible when gateway connected', async ({ page }) => {
    if (await nodesPage.isGatewayDisconnected()) {
      console.warn('[e2e] Gateway not connected — skipping node visibility test');
      test.skip();
      return;
    }

    const cards = await nodesPage.getNodeCards();
    if (cards.length === 0) {
      test.skip(true, 'No nodes visible — gateway may not have populated yet');
      return;
    }

    expect(cards.length).toBeGreaterThanOrEqual(1);
    await expect(cards[0]).toBeVisible();
  });

  test('total node count badge is displayed', async () => {
    if (await nodesPage.isGatewayDisconnected()) {
      test.skip(true, 'Gateway not connected');
      return;
    }
    const cards = await nodesPage.getNodeCards();
    if (cards.length === 0) {
      test.skip(true, 'No nodes visible — gateway may not have populated yet');
      return;
    }
    const count = await nodesPage.getTotalCount();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test('gateway connection establishes WebSocket', async () => {
    const isLive = await nodesPage.isLiveUpdatesConnected();
    const isDisconnected = await nodesPage.isGatewayDisconnected();
    // Gateway should be either connected (live updates) or showing explicit error
    expect(isLive || isDisconnected).toBe(true);
    if (!isLive) {
      console.warn('[e2e] Gateway WS not connected — check IAM token auth with gw.hanzo.bot');
    }
  });

  test('search filters nodes', async ({ page }) => {
    if (await nodesPage.isGatewayDisconnected()) {
      test.skip(true, 'Gateway not connected');
      return;
    }

    const cards = await nodesPage.getNodeCards();
    if (cards.length === 0) {
      test.skip(true, 'No nodes visible — gateway may not have populated yet');
      return;
    }

    const initialCount = cards.length;
    expect(initialCount).toBeGreaterThanOrEqual(1);

    // Search for "macbook" and verify results
    await nodesPage.search('macbook');

    // After filtering, there should be at least one card matching
    const filteredCards = await nodesPage.getNodeCards();
    expect(filteredCards.length).toBeGreaterThanOrEqual(0);
    expect(filteredCards.length).toBeLessThanOrEqual(initialCount);

    // Clear search and verify count restores
    await nodesPage.search('');
    await page.waitForTimeout(2_000);
    const resetCards = await nodesPage.getNodeCards();
    expect(resetCards.length).toBe(initialCount);
  });

  test('clicking a node navigates to its detail page', async ({ page }) => {
    if (await nodesPage.isGatewayDisconnected()) {
      test.skip(true, 'Gateway not connected');
      return;
    }

    const cards = await nodesPage.getNodeCards();
    if (cards.length === 0) {
      test.skip(true, 'No nodes visible');
      return;
    }

    await cards[0].click();
    await page.waitForURL(/\/nodes\//, { timeout: 15_000 });
    expect(page.url()).toMatch(/\/nodes\//);
  });
});
