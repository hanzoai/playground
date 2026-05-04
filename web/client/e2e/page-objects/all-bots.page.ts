import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page object for the Control Plane page (/bots/all).
 * Simple bot card grid with live status, activity toggle, and empty state.
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
  readonly activityToggle: Locator;

  // Bot cards
  readonly botCards: Locator;
  readonly emptyState: Locator;
  readonly createFirstBotButton: Locator;

  // Legacy aliases for backward compat
  readonly agentCards: Locator;
  readonly registerAgentButton: Locator;

  constructor(page: Page) {
    this.page = page;

    this.pageTitle = page.getByRole('heading', { name: 'Control Plane' });
    this.pageDescription = page.getByText(/orchestrate local and cloud agents/i);

    this.liveIndicator = page.locator('span.font-mono.text-green-400', { hasText: 'Live' });
    this.offlineIndicator = page.locator('span.font-mono.text-red-400', { hasText: 'Offline' });
    this.liveUpdatesBadge = this.liveIndicator.or(this.offlineIndicator);

    this.refreshButton = page.locator('button[title="Refresh"]');
    this.activityToggle = page.getByRole('button', { name: /Activity/i });

    // Bot cards — each card is a .border.rounded-lg div in the grid
    this.botCards = page.locator('.grid > div.border');
    this.emptyState = page.getByText(/no bots yet/i);
    this.createFirstBotButton = page.getByRole('button', { name: /create your first bot/i });

    // Legacy aliases — agentCards maps to botCards now
    this.agentCards = this.botCards;
    this.registerAgentButton = this.createFirstBotButton;
  }

  async goto() {
    await this.page.goto('/bots/all', { waitUntil: 'domcontentloaded' });
  }

  async expectPageLoaded() {
    await expect(this.pageTitle).toBeVisible({ timeout: 15_000 });
  }

  async getBotCount(): Promise<number> {
    return this.botCards.count();
  }

  async getAgentCount(): Promise<number> {
    return this.getBotCount();
  }

  async hasAgents(): Promise<boolean> {
    return (await this.getBotCount()) > 0;
  }

  async hasBots(): Promise<boolean> {
    return this.hasAgents();
  }

  /** Click on the first bot card to navigate to its detail page. */
  async clickFirstBot() {
    const firstCard = this.botCards.first();
    await expect(firstCard).toBeVisible({ timeout: 10_000 });
    await firstCard.click();
  }

  /** @deprecated No agent expansion in the new card grid. No-op for compat. */
  async expandAgent(_index: number = 0) {
    // No-op — new UI has flat bot cards, no collapsible agents
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

  /** @deprecated No search input in Control Plane view. No-op. */
  async searchForBot(_query: string) {}

  /** @deprecated No search input in Control Plane view. No-op. */
  async clearSearch() {}

  /** @deprecated No status filter in Control Plane view. No-op. */
  async filterByStatus(_status: 'online' | 'all' | 'offline') {}

  async clickBotByName(name: string) {
    const card = this.botCards.filter({ hasText: name }).first();
    await expect(card).toBeVisible({ timeout: 10_000 });
    await card.click();
  }
}
