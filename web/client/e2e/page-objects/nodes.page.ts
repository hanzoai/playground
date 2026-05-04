import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page object for the Nodes list page (/nodes).
 * Encapsulates selectors and actions for the nodes listing UI.
 */
export class NodesPage {
  readonly page: Page;

  /** Page header title */
  readonly heading: Locator;
  /** Search input */
  readonly searchBar: Locator;
  /** Node cards container */
  readonly nodeList: Locator;
  /** Gateway connection badge */
  readonly connectionBadge: Locator;
  /** Total nodes badge */
  readonly totalBadge: Locator;

  constructor(page: Page) {
    this.page = page;
    this.heading = page.getByRole('heading', { name: 'Nodes' });
    this.searchBar = page.getByPlaceholder(/search nodes/i);
    this.nodeList = page.locator('[data-testid="nodes-list"]').or(
      page.locator('.nodes-virtual-list').or(
        page.locator('[class*="NodesVirtualList"]')
      )
    );
    this.connectionBadge = page.locator('text=Live updates').or(
      page.locator('text=Disconnected')
    );
    this.totalBadge = page.locator('text=/\\d+ total/i');
  }

  async goto() {
    await this.page.goto('/nodes', { waitUntil: 'domcontentloaded' });
  }

  /** Wait for the nodes page to be fully loaded */
  async waitForLoaded() {
    await expect(this.page).toHaveURL(/\/nodes/);
    // Wait for the page to render meaningful content
    // NodeCard renders as <div role="button" aria-label="Navigate to details for node ...">
    const nodeCard = this.page.locator('[role="button"][aria-label*="Navigate to details for node"]');
    const content = nodeCard
      .or(this.page.getByText('Gateway not connected'))
      .or(this.page.getByRole('heading', { name: /nodes/i }));
    await content.first().waitFor({ state: 'visible', timeout: 30_000 });
    // Give gateway WebSocket time to connect and populate node list
    await this.page.waitForTimeout(3_000);
  }

  /** Check if the gateway connection error is showing */
  async isGatewayDisconnected(): Promise<boolean> {
    return this.page.getByText('Gateway not connected').isVisible({ timeout: 2_000 }).catch(() => false);
  }

  /** Click Retry on the gateway error, wait for reconnection */
  async retryGatewayConnection() {
    const retry = this.page.getByRole('button', { name: 'Retry' });
    if (await retry.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await retry.click();
      // Wait for gateway to reconnect and nodes to appear
      await this.page.waitForTimeout(5_000);
    }
  }

  /** Get all visible node card elements */
  async getNodeCards(): Promise<Locator[]> {
    // NodeCard renders as <div role="button" aria-label="Navigate to details for node ...">
    const cards = this.page.locator('[role="button"][aria-label*="Navigate to details for node"]');
    const count = await cards.count();
    const result: Locator[] = [];
    for (let i = 0; i < count; i++) {
      result.push(cards.nth(i));
    }
    return result;
  }

  /** Find a node card by display name or node ID */
  async findNode(nameOrId: string): Promise<Locator | null> {
    const cards = this.page.locator('[role="button"][aria-label*="Navigate to details for node"]');
    const count = await cards.count();
    for (let i = 0; i < count; i++) {
      const text = await cards.nth(i).textContent();
      if (text && text.toLowerCase().includes(nameOrId.toLowerCase())) {
        return cards.nth(i);
      }
    }
    return null;
  }

  /** Extract node ID from a card's aria-label */
  async getNodeIdFromCard(card: Locator): Promise<string> {
    const ariaLabel = await card.getAttribute('aria-label') ?? '';
    // aria-label format: "Navigate to details for node <nodeId>..."
    return ariaLabel
      .replace(/^Navigate to details for node\s*/, '')
      .replace(/[.…]+$/, '')
      .trim();
  }

  /** Click on a node card to navigate to its detail page */
  async openNode(nameOrId: string) {
    const card = await this.findNode(nameOrId);
    if (!card) {
      throw new Error(`Node "${nameOrId}" not found in nodes list`);
    }
    await card.click();
    await this.page.waitForURL(/\/nodes\//, { timeout: 15_000 });
  }

  /** Search for a node */
  async search(query: string) {
    await this.searchBar.fill(query);
    // Wait for filtering and re-render to settle
    await this.page.waitForTimeout(1_500);
  }

  /** Get the total node count from the badge. Returns 0 if badge is not visible. */
  async getTotalCount(): Promise<number> {
    const isVisible = await this.totalBadge.isVisible({ timeout: 5_000 }).catch(() => false);
    if (!isVisible) {
      // Fallback: try the status summary text "X nodes"
      const statusText = this.page.locator('text=/\\d+ nodes?/i');
      const statusVisible = await statusText.isVisible({ timeout: 3_000 }).catch(() => false);
      if (statusVisible) {
        const text = await statusText.textContent();
        const match = text?.match(/(\d+)\s*nodes?/i);
        return match ? parseInt(match[1], 10) : 0;
      }
      // Last fallback: count the node cards
      const cards = await this.getNodeCards();
      return cards.length;
    }
    const text = await this.totalBadge.textContent();
    const match = text?.match(/(\d+)\s*total/i);
    return match ? parseInt(match[1], 10) : 0;
  }

  /** Check if "Live updates" badge is visible (gateway connected) */
  async isLiveUpdatesConnected(): Promise<boolean> {
    return this.page.locator('text=Live updates').isVisible({ timeout: 5_000 }).catch(() => false);
  }
}

/**
 * Page object for the Node Detail page (/nodes/:nodeId).
 * Handles tabs: Overview, Terminal, Desktop, MCP Servers, etc.
 */
export class NodeDetailPage {
  readonly page: Page;

  /** Node display name / header */
  readonly nodeHeader: Locator;
  /** Tab triggers */
  readonly overviewTab: Locator;
  readonly terminalTab: Locator;
  readonly desktopTab: Locator;
  readonly mcpServersTab: Locator;
  readonly toolsTab: Locator;
  readonly configurationTab: Locator;
  /** Back button */
  readonly backButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.nodeHeader = page.locator('[class*="NodeDetailHeader"], [data-testid="node-header"]').or(
      page.locator('header').first()
    );
    this.overviewTab = page.getByRole('tab', { name: 'Overview' });
    this.terminalTab = page.getByRole('tab', { name: 'Terminal' });
    this.desktopTab = page.getByRole('tab', { name: 'Desktop' });
    this.mcpServersTab = page.getByRole('tab', { name: 'MCP Servers' });
    this.toolsTab = page.getByRole('tab', { name: 'Tools' });
    this.configurationTab = page.getByRole('tab', { name: 'Configuration' });
    this.backButton = page.getByRole('button', { name: /back/i }).or(
      page.locator('button[aria-label*="back" i]').or(
        page.locator('[data-testid="back-button"]')
      )
    );
  }

  async goto(nodeId: string) {
    await this.page.goto(`/nodes/${encodeURIComponent(nodeId)}`, { waitUntil: 'domcontentloaded' });
  }

  /** Wait for node detail page to load */
  async waitForLoaded() {
    await expect(this.page).toHaveURL(/\/nodes\//);
    // Wait for tabs to render — all node detail pages have at least Overview tab
    await this.overviewTab.waitFor({ state: 'visible', timeout: 30_000 });
  }

  /** Get the node ID from the overview card */
  async getNodeId(): Promise<string> {
    // Node ID is displayed in the overview card under "Node ID"
    const nodeIdEl = this.page.locator('dt:text("Node ID") + dd, dd:has-text("antje-macbook")');
    const text = await nodeIdEl.first().textContent();
    return text?.trim() ?? '';
  }

  /** Switch to the Terminal tab */
  async switchToTerminal() {
    await this.terminalTab.click();
    await this.page.waitForURL(/#terminal/, { timeout: 5_000 }).catch(() => {
      // Hash navigation may not trigger URL change event
    });
    // Wait for the terminal container to appear
    await this.page.waitForSelector('.xterm, [class*="xterm"], [class*="TerminalPanel"]', {
      timeout: 15_000,
    });
  }

  /** Switch to the Desktop tab */
  async switchToDesktop() {
    await this.desktopTab.click();
    await this.page.waitForURL(/#desktop/, { timeout: 5_000 }).catch(() => {});
    // Wait for either the VNC iframe or the "not available" message
    const desktopContent = this.page.locator('iframe[title*="Desktop"]')
      .or(this.page.getByText('Desktop not available'))
      .or(this.page.getByText('Enable Screen Sharing'));
    await desktopContent.first().waitFor({ state: 'visible', timeout: 15_000 });
  }

  /** Check if terminal panel is visible and initialized */
  async isTerminalVisible(): Promise<boolean> {
    return this.page.locator('.xterm, [class*="xterm"]').isVisible({ timeout: 10_000 }).catch(() => false);
  }

  /** Get the terminal connection status text */
  async getTerminalConnectionStatus(): Promise<string> {
    const indicator = this.page.locator('text=/Connected|Connecting|Disconnected|Error/').first();
    return (await indicator.textContent())?.trim() ?? 'unknown';
  }

  /** Check if desktop panel has a VNC iframe or shows unavailable message */
  async getDesktopState(): Promise<'iframe' | 'unavailable'> {
    const iframe = this.page.locator('iframe[title*="Desktop"]');
    if (await iframe.isVisible({ timeout: 5_000 }).catch(() => false)) {
      return 'iframe';
    }
    return 'unavailable';
  }

  /** Get the overview tab's Node Information card fields */
  async getNodeInfo(): Promise<Record<string, string>> {
    await this.overviewTab.click();
    const info: Record<string, string> = {};
    const dts = this.page.locator('dt');
    const count = await dts.count();
    for (let i = 0; i < count; i++) {
      const label = (await dts.nth(i).textContent())?.trim() ?? '';
      const dd = dts.nth(i).locator('+ dd');
      const value = (await dd.textContent())?.trim() ?? '';
      if (label) {
        info[label] = value;
      }
    }
    return info;
  }
}
