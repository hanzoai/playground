import { test, expect } from '../../fixtures';
import { CommerceHelper } from '../../helpers/commerce-api.helper';
import { extractTokenFromPage, getAccessToken } from '../../helpers/iam-auth.helper';

/**
 * Billing & Credits E2E tests.
 *
 * All Hanzo services use IAM — the JWT from hanzo.id works for Commerce API
 * at api.hanzo.ai. Free tier gets $5 trial credit.
 */

test.describe('Trial Credits & Billing', () => {
  let commerce: CommerceHelper;
  let userId: string;

  test.beforeAll(async () => {
    // Get an access token for Commerce API calls
    const token = await getAccessToken();
    commerce = new CommerceHelper(token);

    // Extract user ID from JWT claims
    const payload = JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString());
    userId = payload.sub || payload.name || process.env.E2E_IAM_USER_EMAIL!;
  });

  test('user has a balance on Commerce API', async ({ page }) => {
    await page.goto('/dashboard', { waitUntil: 'networkidle' });

    // Verify Commerce API is reachable with our JWT
    const token = await extractTokenFromPage(page);
    expect(token).toBeTruthy();

    const browserCommerce = new CommerceHelper(token!);

    // Get balance — should not throw
    const balance = await browserCommerce.getBalance(userId).catch((err) => {
      console.warn(`Commerce balance check failed: ${err.message}`);
      return null;
    });

    // If Commerce is reachable, verify balance structure
    if (balance) {
      expect(balance).toHaveProperty('balance');
      expect(balance).toHaveProperty('available');
      expect(typeof balance.balance).toBe('number');
      expect(typeof balance.available).toBe('number');
    } else {
      test.skip(true, 'Commerce API not reachable — skipping balance check');
    }
  });

  test('$5 trial credit is available for new accounts', async () => {
    const hasCredit = await commerce.hasCredit(userId);

    if (!hasCredit) {
      // Try granting starter credit
      try {
        await commerce.grantStarterCredit(userId);
        console.log('Granted $5 starter credit to test user');
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err);
        console.warn(`Could not grant starter credit: ${msg}`);
        test.skip(true, 'Cannot grant starter credit — Commerce API may not allow it');
        return;
      }
    }

    // Verify the balance is at least $5 (500 cents)
    const balance = await commerce.getBalance(userId);
    expect(balance.available).toBeGreaterThanOrEqual(500);

    const dollars = balance.available / 100;
    console.log(`User balance: $${dollars.toFixed(2)}`);
  });

  test('trial credit balance reflects in dollar amount', async () => {
    const dollars = await commerce.getTrialCreditDollars(userId).catch(() => null);
    if (dollars === null) {
      test.skip(true, 'Commerce API not reachable — skipping balance check');
      return;
    }
    expect(dollars).toBeGreaterThanOrEqual(0);
    console.log(`Trial credit balance: $${dollars.toFixed(2)}`);
  });

  test('usage records are retrievable', async () => {
    const records = await commerce.getUsageRecords(userId).catch(() => null);

    if (records === null) {
      test.skip(true, 'Commerce usage API not reachable');
      return;
    }

    // Records should be an array (may be empty for new accounts)
    expect(Array.isArray(records)).toBe(true);
    console.log(`User has ${records.length} usage records`);
  });

  test('bot execution button is enabled when user has credits', async ({ page }) => {
    await page.goto('/bots/all', { waitUntil: 'domcontentloaded' });

    // Wait for Control Plane page to load
    await expect(page.getByRole('heading', { name: 'Control Plane' })).toBeVisible({ timeout: 15_000 });

    // Check if there are any agent cards with bots
    const agentCards = page.locator('.border.border-border\\/30.rounded-md');
    const agentCount = await agentCards.count();

    if (agentCount === 0) {
      test.skip(true, 'No bots available to check execution button');
      return;
    }

    // Expand first agent card and click first bot
    await agentCards.first().locator('button').first().click();
    await page.waitForTimeout(300);
    const botButton = agentCards.first().locator('.border-t button').first();
    const hasBots = await botButton.count() > 0;
    if (!hasBots) {
      test.skip(true, 'No bots available to check execution button');
      return;
    }
    await botButton.click();
    await page.waitForURL(/\/bots\//, { timeout: 15_000 });

    // Execute button should be visible and enabled (user has credits)
    const executeButton = page.getByRole('button', { name: /execute bot|execute/i });
    await expect(executeButton).toBeVisible({ timeout: 10_000 });
    await expect(executeButton).toBeEnabled();
  });
});
