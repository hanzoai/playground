import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page object for the Control Plane page (/bots/all).
 * Agent network view, live status, agent cards with nested bots, activity stream.
 */
export class AllBotsPage {
  readonly page: Page;

  // Header
  readonly pageTitle: Locator;
  readonly pageDescription: Locator;

  // Status indicator
  readonly liveIndicator: Locator;
  readonly offlineIndicator: Locator;
  readonly liveUpdatesBadge: Locator;

  // Actions
  readonly refreshButton: Locator;
  readonly registerAgentButton: Locator;

  // Metrics
  readonly metricsStrip: Locator;

  // Agent cards
  readonly agentCards: Locator;
  readonly emptyState: Locator;

  // Activity
  readonly activityStream: Locator;

  constructor(page: Page) {
    this.page = page;

    this.pageTitle = page.getByRole('heading', { name: 'Control Plane' });
    this.pageDescription = page.getByText(/orchestrate local and cloud agents/i);

    this.liveIndicator = page.locator('span.font-mono.text-green-400', { hasText: 'Live' });
    this.offlineIndicator = page.locator('span.font-mono.text-red-400', { hasText: 'Offline' });
    this.liveUpdatesBadge = this.liveIndicator.or(this.offlineIndicator);

    this.refreshButton = page.locator('button[title="Refresh"]');
    this.registerAgentButton = page.getByText('Register Agent');

    this.metricsStrip = page.locator('.flex.items-center.gap-4.py-2');

    this.agentCards = page.locator('.border.border-border\\/30.rounded-md');
    this.emptyState = page.getByText(/no agents connected/i);

    this.activityStream = page.getByText('Activity');
  }

  async goto() {
    // Use 'domcontentloaded' — the page has SSE connections that prevent 'networkidle'
    await this.page.goto('/bots/all', { waitUntil: 'domcontentloaded' });
  }

  async expectPageLoaded() {
    await expect(this.pageTitle).toBeVisible({ timeout: 15_000 });
  }

  async getAgentCount(): Promise<number> {
    return this.agentCards.count();
  }

  async hasAgents(): Promise<boolean> {
    return (await this.getAgentCount()) > 0;
  }

  /** Expand an agent card to reveal its bots. */
  async expandAgent(index: number = 0) {
    const card = this.agentCards.nth(index);
    await expect(card).toBeVisible({ timeout: 10_000 });
    const header = card.locator('button').first();
    await header.click();
    // Wait for expansion animation
    await this.page.waitForTimeout(300);
  }

  /** Get all bot buttons inside an expanded agent card. */
  getBotButtons(agentIndex: number = 0): Locator {
    return this.agentCards.nth(agentIndex).locator('button.w-full.flex.items-center').filter({ hasNot: this.page.locator('svg') });
  }

  /** Click the first bot inside the first agent card. */
  async clickFirstBot() {
    // Expand first agent if not already expanded
    const firstAgent = this.agentCards.first();
    await expect(firstAgent).toBeVisible({ timeout: 10_000 });

    // Check if agent has expanded bot list (border-t border-border/20 div)
    const botList = firstAgent.locator('.border-t');
    const isExpanded = await botList.count() > 0;
    if (!isExpanded) {
      await this.expandAgent(0);
    }

    // Click first bot button inside the expanded area
    const botButton = firstAgent.locator('.border-t button').first();
    await expect(botButton).toBeVisible({ timeout: 5_000 });
    await botButton.click();
  }

  async expectSSEConnected() {
    await expect(this.liveIndicator).toBeVisible({ timeout: 15_000 });
  }

  async expectSSEDisconnected() {
    await expect(this.offlineIndicator).toBeVisible();
  }

  async refresh() {
    await this.refreshButton.first().click();
    await this.page.waitForTimeout(1_000);
  }

  /** @deprecated No search input in Control Plane view. Kept for backward compat — no-ops. */
  async searchForBot(_query: string) {
    // Control Plane view has no search input
  }

  /** @deprecated No search input in Control Plane view. Kept for backward compat — no-ops. */
  async clearSearch() {
    // Control Plane view has no search input
  }

  /** @deprecated No status filter in Control Plane view. Kept for backward compat — no-ops. */
  async filterByStatus(_status: 'online' | 'all' | 'offline') {
    // Control Plane view has no status filter buttons
  }

  /** Check if bots exist (agents with bots inside). */
  async hasBots(): Promise<boolean> {
    const hasAgents = await this.hasAgents();
    if (!hasAgents) return false;

    // Check if any agent card mentions bots
    const botCountTexts = this.page.locator('text=/\\d+ bots?/');
    const count = await botCountTexts.count();
    return count > 0;
  }

  async getBotCount(): Promise<number> {
    return this.agentCards.count();
  }

  async clickBotByName(name: string) {
    // Expand all agents to find the bot
    const agentCount = await this.getAgentCount();
    for (let i = 0; i < agentCount; i++) {
      await this.expandAgent(i);
      const botButton = this.agentCards.nth(i).locator('.border-t button', { hasText: name }).first();
      if (await botButton.count() > 0) {
        await botButton.click();
        return;
      }
    }
    throw new Error(`Bot "${name}" not found in any agent card`);
  }
}
