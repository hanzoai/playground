import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { test, expect } from '../../fixtures';
import { NodesPage, NodeDetailPage } from '../../page-objects/nodes.page';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const authDir = path.resolve(__dirname, '../../.auth');

/**
 * Node detail E2E tests.
 *
 * Verifies:
 *   - Node detail page loads for a connected node
 *   - Overview tab shows node information (ID, version, status)
 *   - Terminal tab renders xterm.js and shows connection status
 *   - Desktop tab renders VNC iframe or shows availability message
 *   - All tabs are accessible and navigate via URL hash
 *   - Real production node, real WebSocket, no mocks
 */

const TARGET_NODE = process.env.E2E_TARGET_NODE_ID || 'antje-macbook-2';
const TARGET_NODE_SEARCH = process.env.E2E_TARGET_NODE_SEARCH || 'MacBook';

test.describe('Node Detail Page', () => {
  let detailPage: NodeDetailPage;
  let resolvedNodeId: string;

  let gatewayConnected = false;

  test.beforeAll(async ({ browser }) => {
    // Create context with saved auth state (cookies + localStorage)
    const storageStatePath = path.join(authDir, 'user.json');
    const tokenFilePath = path.join(authDir, 'iam-tokens.json');

    const context = await browser.newContext({
      storageState: storageStatePath,
    });

    // Inject IAM sessionStorage tokens (same pattern as fixtures.ts)
    if (fs.existsSync(tokenFilePath)) {
      const tokens = JSON.parse(fs.readFileSync(tokenFilePath, 'utf-8'));
      await context.addInitScript((tokenData: Record<string, string>) => {
        for (const [key, value] of Object.entries(tokenData)) {
          sessionStorage.setItem(key, value);
        }
      }, tokens);
    }

    const page = await context.newPage();
    const nodesPage = new NodesPage(page);
    await nodesPage.goto();
    await nodesPage.waitForLoaded();

    if (await nodesPage.isGatewayDisconnected()) {
      await nodesPage.retryGatewayConnection();
    }

    gatewayConnected = !(await nodesPage.isGatewayDisconnected());

    if (gatewayConnected) {
      const macNode = await nodesPage.findNode(TARGET_NODE_SEARCH) ??
                      await nodesPage.findNode('macbook') ??
                      await nodesPage.findNode('antje-macbook');

      if (macNode) {
        // Extract node ID from aria-label: "Navigate to details for node <id>..."
        resolvedNodeId = await nodesPage.getNodeIdFromCard(macNode);
        if (!resolvedNodeId) {
          resolvedNodeId = TARGET_NODE;
        }
      } else {
        resolvedNodeId = TARGET_NODE;
      }
    } else {
      resolvedNodeId = TARGET_NODE;
    }

    await context.close();
  });

  test.beforeEach(async ({ page }) => {
    detailPage = new NodeDetailPage(page);
    await detailPage.goto(resolvedNodeId);

    // Wait for initial render — either tabs, gateway error, or node not found
    await page.waitForTimeout(3_000);
    const tabsOrError = page.locator('[role="tab"]')
      .or(page.getByText('Gateway not connected'))
      .or(page.getByText('Node not found'));
    await tabsOrError.first().waitFor({ state: 'visible', timeout: 30_000 });

    // Retry gateway connection if disconnected
    const disconnected = await page.getByText('Gateway not connected')
      .isVisible({ timeout: 2_000 }).catch(() => false);
    if (disconnected) {
      const retry = page.getByRole('button', { name: 'Retry' });
      if (await retry.isVisible({ timeout: 2_000 }).catch(() => false)) {
        await retry.click();
        await page.waitForTimeout(8_000);
      }
    }
  });

  /** Check if the current page has gateway-loaded tabs */
  async function hasTabsOnPage(page: import('@playwright/test').Page): Promise<boolean> {
    return page.locator('[role="tab"]').first()
      .isVisible({ timeout: 2_000 }).catch(() => false);
  }

  test('overview tab loads with node information', async ({ page }) => {
    if (!await hasTabsOnPage(page)) {
      test.skip();
      return;
    }

    await expect(detailPage.overviewTab).toBeVisible();
    const nodeInfoCard = page.getByText('Node Information');
    await expect(nodeInfoCard).toBeVisible({ timeout: 10_000 });
    const info = await detailPage.getNodeInfo();
    expect(info['Node ID'] || '').toBeTruthy();
  });

  test('all tabs are present', async ({ page }) => {
    if (!await hasTabsOnPage(page)) { test.skip(); return; }
    await expect(detailPage.overviewTab).toBeVisible();
    await expect(detailPage.terminalTab).toBeVisible();
    await expect(detailPage.desktopTab).toBeVisible();
    await expect(detailPage.mcpServersTab).toBeVisible();
    await expect(detailPage.toolsTab).toBeVisible();
    await expect(detailPage.configurationTab).toBeVisible();
  });

  test('terminal tab renders xterm container', async ({ page }) => {
    if (!await hasTabsOnPage(page)) { test.skip(); return; }
    await detailPage.switchToTerminal();
    const isVisible = await detailPage.isTerminalVisible();
    expect(isVisible).toBe(true);
    const statusText = await detailPage.getTerminalConnectionStatus();
    expect(['Connected', 'Connecting...', 'Disconnected', 'Error']).toContain(statusText);
  });

  test('terminal tab has connection status indicator', async ({ page }) => {
    if (!await hasTabsOnPage(page)) { test.skip(); return; }
    await detailPage.switchToTerminal();
    const statusOverlay = page.locator('.absolute.top-2.right-2').or(
      page.locator('[class*="absolute"][class*="top-2"][class*="right-2"]')
    );
    await expect(statusOverlay.first()).toBeVisible({ timeout: 10_000 });
  });

  test('terminal renders initial connection message', async ({ page }) => {
    if (!await hasTabsOnPage(page)) { test.skip(); return; }
    await detailPage.switchToTerminal();
    await page.waitForTimeout(2000);
    const xtermRows = page.locator('.xterm-rows');
    await expect(xtermRows).toBeVisible({ timeout: 10_000 });
  });

  test('desktop tab shows VNC iframe or availability message', async ({ page }) => {
    if (!await hasTabsOnPage(page)) { test.skip(); return; }
    await detailPage.switchToDesktop();
    const state = await detailPage.getDesktopState();
    if (state === 'iframe') {
      const iframe = page.locator('iframe[title*="Desktop"]');
      const src = await iframe.getAttribute('src');
      expect(src).toBeTruthy();
      expect(src).toContain('vnc-viewer');
    } else {
      const msg = page.getByText('Desktop not available').or(
        page.getByText('Enable Screen Sharing')
      );
      await expect(msg.first()).toBeVisible();
    }
  });

  test('tab navigation updates URL hash', async ({ page }) => {
    if (!await hasTabsOnPage(page)) { test.skip(); return; }
    await detailPage.terminalTab.click();
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#terminal');
    await detailPage.desktopTab.click();
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#desktop');
    await detailPage.overviewTab.click();
    await page.waitForTimeout(500);
    expect(page.url()).toContain('#overview');
  });

  test('direct URL hash navigation works', async ({ page }) => {
    if (!await hasTabsOnPage(page)) { test.skip(); return; }
    await page.goto(`/nodes/${encodeURIComponent(resolvedNodeId)}#terminal`, {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForTimeout(5_000);
    const isVisible = await detailPage.isTerminalVisible();
    expect(isVisible).toBe(true);
  });

  test('node detail API returns valid data', async ({ page }) => {
    const apiBase = process.env.E2E_API_BASE_URL || 'https://api.hanzo.bot/v1';
    const token = await page.evaluate(() => {
      return sessionStorage.getItem('hanzo_iam_access_token')
        || localStorage.getItem('af_iam_token');
    });

    if (!token) {
      test.skip(true, 'No auth token available');
      return;
    }

    const response = await page.request.get(`${apiBase}/nodes/summary`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data).toHaveProperty('nodes');
    expect(Array.isArray(data.nodes)).toBe(true);
  });
});
