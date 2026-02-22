import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page object for the All Bots page (/bots/all).
 * Bot grid, SSE live updates, search/filter, bot card navigation.
 */
export class AllBotsPage {
  readonly page: Page;

  // Header
  readonly pageTitle: Locator;
  readonly pageDescription: Locator;

  // View controls
  readonly gridViewButton: Locator;
  readonly tableViewButton: Locator;
  readonly liveUpdatesBadge: Locator;
  readonly refreshButton: Locator;

  // Search & filters
  readonly searchInput: Locator;
  readonly onlineFilterButton: Locator;
  readonly allFilterButton: Locator;
  readonly offlineFilterButton: Locator;
  readonly clearFiltersLink: Locator;

  // Content
  readonly botCards: Locator;
  readonly loadMoreButton: Locator;
  readonly emptyState: Locator;

  constructor(page: Page) {
    this.page = page;

    this.pageTitle = page.getByText('All Bots', { exact: false });
    this.pageDescription = page.getByText(/browse and execute bots/i);

    this.gridViewButton = page.getByLabel(/grid/i).or(page.locator('[value="grid"]'));
    this.tableViewButton = page.getByLabel(/table|list/i).or(page.locator('[value="table"]'));
    this.liveUpdatesBadge = page.getByText(/live updates|disconnected/i);
    this.refreshButton = page.getByRole('button', { name: /refresh/i });

    this.searchInput = page.locator('input[placeholder*="Search bots" i]');
    this.onlineFilterButton = page.getByRole('button', { name: /^online\b/i });
    this.allFilterButton = page.getByRole('button', { name: /^all\b/i });
    this.offlineFilterButton = page.getByRole('button', { name: /^offline\b/i });
    this.clearFiltersLink = page.getByText(/clear filters/i);

    this.botCards = page.locator('[role="button"][aria-label*="View bot"]');
    this.loadMoreButton = page.getByRole('button', { name: /load more/i });
    this.emptyState = page.getByText(/no bots available|no online bots|no offline bots/i);
  }

  async goto() {
    // Use 'domcontentloaded' — the bots page has SSE connections that prevent 'networkidle'
    await this.page.goto('/bots/all', { waitUntil: 'domcontentloaded' });
  }

  async expectPageLoaded() {
    await expect(this.pageTitle).toBeVisible({ timeout: 15_000 });
  }

  async getBotCount(): Promise<number> {
    return this.botCards.count();
  }

  async hasBots(): Promise<boolean> {
    return (await this.getBotCount()) > 0;
  }

  async searchForBot(query: string) {
    await this.searchInput.fill(query);
    // Wait for filter to take effect
    await this.page.waitForTimeout(500);
  }

  async clearSearch() {
    await this.searchInput.clear();
    await this.page.waitForTimeout(500);
  }

  async filterByStatus(status: 'online' | 'all' | 'offline') {
    const button = {
      online: this.onlineFilterButton,
      all: this.allFilterButton,
      offline: this.offlineFilterButton,
    }[status];
    await button.click();
    await this.page.waitForTimeout(500);
  }

  async clickFirstBot() {
    const firstBot = this.botCards.first();
    await expect(firstBot).toBeVisible({ timeout: 10_000 });
    await firstBot.click();
  }

  async clickBotByName(name: string) {
    const bot = this.page.locator(`[role="button"][aria-label*="${name}"]`).first();
    await expect(bot).toBeVisible({ timeout: 10_000 });
    await bot.click();
  }

  async expectSSEConnected() {
    await expect(this.page.getByText(/live updates/i)).toBeVisible({ timeout: 15_000 });
  }

  async expectSSEDisconnected() {
    await expect(this.page.getByText(/disconnected/i)).toBeVisible();
  }

  async refresh() {
    await this.refreshButton.first().click();
    await this.page.waitForTimeout(1_000);
  }
}
