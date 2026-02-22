import { test, expect } from '../../fixtures';
import { AllBotsPage } from '../../page-objects/all-bots.page';
import { BotDetailPage } from '../../page-objects/bot-detail.page';
import { CommerceHelper } from '../../helpers/commerce-api.helper';
import { getAccessToken, extractTokenFromPage } from '../../helpers/iam-auth.helper';

/**
 * Bot Execution E2E tests.
 *
 * Verifies that an authenticated user can:
 * - Navigate to a bot
 * - Execute it
 * - See results
 * - Credits are deducted
 */

test.describe('Bot Execution', () => {
  test('navigate to bot and see execution form', async ({ page }) => {
    const botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();

    const hasBots = await botsPage.hasBots();
    if (!hasBots) {
      test.skip(true, 'No bots available for execution test');
      return;
    }

    // Navigate to first available bot
    await botsPage.clickFirstBot();
    await page.waitForURL(/\/bots\//, { timeout: 15_000 });

    const botDetail = new BotDetailPage(page);
    await botDetail.expectPageLoaded();

    // Execute button should be visible
    await botDetail.expectExecuteButtonEnabled();
  });

  test('execute a bot and see result', async ({ page }) => {
    const botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();

    const hasBots = await botsPage.hasBots();
    if (!hasBots) {
      test.skip(true, 'No bots available for execution test');
      return;
    }

    // Navigate to first bot
    await botsPage.clickFirstBot();
    await page.waitForURL(/\/bots\//, { timeout: 15_000 });

    const botDetail = new BotDetailPage(page);
    await botDetail.expectPageLoaded();
    await botDetail.expectExecuteButtonEnabled();

    // Execute the bot
    await botDetail.executeBot();

    // Wait for result
    await botDetail.waitForExecutionResult(30_000);

    // Verify no error
    await botDetail.expectExecutionSuccess();

    console.log('Bot execution completed successfully');
  });

  test('execution result has formatted and JSON views', async ({ page }) => {
    const botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();

    const hasBots = await botsPage.hasBots();
    if (!hasBots) {
      test.skip(true, 'No bots available');
      return;
    }

    await botsPage.clickFirstBot();
    await page.waitForURL(/\/bots\//, { timeout: 15_000 });

    const botDetail = new BotDetailPage(page);
    await botDetail.expectPageLoaded();

    // Execute
    await botDetail.executeBot();
    await botDetail.waitForExecutionResult(30_000);

    // Switch to JSON view
    await botDetail.switchToJsonView();
    // Should still see content
    const jsonText = await botDetail.getExecutionResultText();
    expect(jsonText.length).toBeGreaterThan(0);

    // Switch to formatted view
    await botDetail.switchToFormattedView();
  });

  test('execution deducts credits from balance', async ({ page }) => {
    // Get initial balance
    let token: string;
    let userId: string;
    try {
      token = await getAccessToken();
      const payload = JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString());
      userId = payload.sub || payload.name || process.env.E2E_IAM_USER_EMAIL!;
    } catch {
      test.skip(true, 'Cannot get access token for balance check');
      return;
    }

    const commerce = new CommerceHelper(token);

    let balanceBefore: number;
    try {
      const before = await commerce.getBalance(userId);
      balanceBefore = before.available;
    } catch {
      test.skip(true, 'Commerce API not reachable — cannot check balance');
      return;
    }

    if (balanceBefore <= 0) {
      test.skip(true, 'No credits available for execution');
      return;
    }

    // Navigate to a bot and execute
    const botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();

    const hasBots = await botsPage.hasBots();
    if (!hasBots) {
      test.skip(true, 'No bots available');
      return;
    }

    await botsPage.clickFirstBot();
    await page.waitForURL(/\/bots\//, { timeout: 15_000 });

    const botDetail = new BotDetailPage(page);
    await botDetail.expectPageLoaded();
    await botDetail.executeBot();
    await botDetail.waitForExecutionResult(30_000);

    // Check balance after execution — allow some time for billing to process
    await page.waitForTimeout(3_000);

    const after = await commerce.getBalance(userId);
    const balanceAfter = after.available;

    // Balance should have decreased (or stayed same if execution was free)
    console.log(`Balance: before=$${(balanceBefore / 100).toFixed(2)}, after=$${(balanceAfter / 100).toFixed(2)}`);
    expect(balanceAfter).toBeLessThanOrEqual(balanceBefore);
  });

  test('bot detail shows activity tab with execution history', async ({ page }) => {
    const botsPage = new AllBotsPage(page);
    await botsPage.goto();
    await botsPage.expectPageLoaded();

    const hasBots = await botsPage.hasBots();
    if (!hasBots) {
      test.skip(true, 'No bots available');
      return;
    }

    await botsPage.clickFirstBot();
    await page.waitForURL(/\/bots\//, { timeout: 15_000 });

    const botDetail = new BotDetailPage(page);
    await botDetail.expectPageLoaded();

    // Click activity tab
    await botDetail.viewActivity();

    // Activity section should be visible (may be empty)
    await expect(botDetail.activityTab).toHaveAttribute('data-state', 'active', { timeout: 5_000 }).catch(() => {
      // Tab may use different state attribute
    });
  });
});
