import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page object for the Bot Detail page (/bots/:fullBotId).
 * Execution form, result display, metrics, activity.
 */
export class BotDetailPage {
  readonly page: Page;

  // Header
  readonly botName: Locator;
  readonly botDescription: Locator;
  readonly statusIndicator: Locator;
  readonly copyCurlButton: Locator;

  // Execution
  readonly executeButton: Locator;
  readonly executingIndicator: Locator;

  // Schema tabs
  readonly inputSchemaTab: Locator;
  readonly outputSchemaTab: Locator;

  // Results
  readonly formattedViewTab: Locator;
  readonly jsonViewTab: Locator;
  readonly executionResult: Locator;
  readonly executionError: Locator;

  // Activity & Performance tabs
  readonly activityTab: Locator;
  readonly performanceTab: Locator;

  // Stats cards
  readonly avgResponseTime: Locator;
  readonly successRate: Locator;
  readonly totalExecutions: Locator;

  // Loading & error states
  readonly loadingSpinner: Locator;
  readonly errorAlert: Locator;

  constructor(page: Page) {
    this.page = page;

    this.botName = page.locator('h2.text-display');
    this.botDescription = page.locator('h2.text-display + p.text-body');
    this.statusIndicator = page.locator('[class*="status"]').first();
    this.copyCurlButton = page.getByRole('button', { name: /copy curl/i });

    this.executeButton = page.getByRole('button', { name: 'Execute Bot' });
    this.executingIndicator = page.getByText(/executing\.\.\./i);

    this.inputSchemaTab = page.getByRole('tab', { name: /input schema/i });
    this.outputSchemaTab = page.getByRole('tab', { name: /output schema/i });

    this.formattedViewTab = page.getByText(/formatted/i);
    this.jsonViewTab = page.getByText(/^json$/i);
    this.executionResult = page.locator('[class*="result"], [class*="output"]').first();
    this.executionError = page.locator('[class*="destructive"], [class*="error"]').first();

    this.activityTab = page.getByRole('tab', { name: /activity/i });
    this.performanceTab = page.getByRole('tab', { name: /performance/i });

    this.avgResponseTime = page.getByText(/avg.*response/i);
    this.successRate = page.getByText(/success.*rate/i);
    this.totalExecutions = page.getByText(/total.*executions/i);

    this.loadingSpinner = page.locator('.animate-spin');
    this.errorAlert = page.locator('[role="alert"]');
  }

  async expectPageLoaded() {
    // Wait for either bot name heading or error alert to appear
    const loaded = await Promise.race([
      this.botName.waitFor({ state: 'visible', timeout: 15_000 }).then(() => true).catch(() => false),
      this.errorAlert.first().waitFor({ state: 'visible', timeout: 15_000 }).then(() => true).catch(() => false),
    ]);
    if (!loaded) {
      throw new Error('Bot detail page did not load — neither bot name nor error alert visible');
    }
  }

  async expectBotNameVisible(name?: string) {
    if (name) {
      await expect(this.page.getByText(name)).toBeVisible({ timeout: 10_000 });
    } else {
      await expect(this.botName).toBeVisible({ timeout: 10_000 });
    }
  }

  async expectExecuteButtonEnabled() {
    await expect(this.executeButton).toBeVisible();
    await expect(this.executeButton).toBeEnabled();
  }

  async expectExecuteButtonDisabled() {
    await expect(this.executeButton).toBeDisabled();
  }

  /**
   * Execute the bot with optional JSON input.
   * If the bot has an input form, fill it first.
   */
  async executeBot(input?: Record<string, unknown>) {
    if (input) {
      // If there's a JSON editor or form fields, fill them
      const jsonInput = this.page.locator('textarea, [contenteditable="true"]').first();
      if (await jsonInput.isVisible({ timeout: 2_000 }).catch(() => false)) {
        await jsonInput.fill(JSON.stringify(input));
      }
    }

    await this.executeButton.click();
  }

  /**
   * Wait for execution to complete (spinner gone, result visible).
   */
  async waitForExecutionResult(timeout: number = 30_000) {
    // Wait for "Executing..." to appear, then disappear
    try {
      await this.executingIndicator.waitFor({ state: 'visible', timeout: 5_000 });
    } catch {
      // May already be done
    }
    await this.executingIndicator.waitFor({ state: 'hidden', timeout });

    // Result or error should be visible
    await expect(
      this.page.getByText(/formatted|json|result|output|error|failed/i).first()
    ).toBeVisible({ timeout: 10_000 });
  }

  async expectExecutionSuccess() {
    // No error alert should be visible
    const errorVisible = await this.executionError.isVisible().catch(() => false);
    if (errorVisible) {
      const errorText = await this.executionError.textContent();
      throw new Error(`Execution failed with error: ${errorText}`);
    }
  }

  async switchToJsonView() {
    await this.jsonViewTab.click();
  }

  async switchToFormattedView() {
    await this.formattedViewTab.click();
  }

  async getExecutionResultText(): Promise<string> {
    return (await this.executionResult.textContent()) || '';
  }

  async viewActivity() {
    await this.activityTab.click();
  }

  async viewPerformance() {
    await this.performanceTab.click();
  }
}
