import { test, expect } from '../../fixtures';
import { CommerceHelper } from '../../helpers/commerce-api.helper';
import { extractTokenFromPage, getAccessToken } from '../../helpers/iam-auth.helper';

/**
 * Paid Launch Flow E2E tests.
 *
 * Exercises the /launch page end-to-end: preset rendering, balance display,
 * insufficient-funds gating, billing error alerts, and successful provisioning.
 *
 * Auth: IAM JWT is injected into sessionStorage by the fixture (see fixtures.ts).
 * Billing: checked via Commerce API at commerce.hanzo.ai.
 */

/** Minimum preset cost in cents (Starter = $0.02/hr). */
const STARTER_CENTS_PER_HOUR = 2;

/** Known preset names in display order. */
const PRESET_NAMES = ['Starter', 'Pro', 'Power', 'GPU'] as const;

/** Expected spec details for preset cards. */
const PRESET_SPECS: Record<string, { vcpus: number; memoryGB: number }> = {
  Starter: { vcpus: 1, memoryGB: 2 },
  Pro:     { vcpus: 2, memoryGB: 4 },
  Power:   { vcpus: 4, memoryGB: 8 },
  GPU:     { vcpus: 2, memoryGB: 8 },
};

/**
 * Navigate to /launch and wait for hydration + balance fetch.
 */
async function navigateToLaunch(page: import('@playwright/test').Page) {
  await page.goto('/launch', { waitUntil: 'domcontentloaded' });
  await page.waitForLoadState('networkidle').catch(() => {});
  await page.waitForTimeout(3_000);
}

/**
 * Obtain a Commerce helper and userId from the current page context,
 * falling back to the saved auth token file.  Returns null fields when
 * auth or Commerce is unavailable so callers can skip gracefully.
 */
async function resolveCommerceContext(page: import('@playwright/test').Page): Promise<{
  commerce: CommerceHelper | null;
  userId: string | null;
  token: string | null;
}> {
  let token = await extractTokenFromPage(page);
  if (!token) {
    try {
      token = await getAccessToken();
    } catch {
      return { commerce: null, userId: null, token: null };
    }
  }

  const payload = JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString());
  const userId: string = payload.sub || payload.name || process.env.E2E_IAM_USER_EMAIL!;
  const commerce = new CommerceHelper(token);

  return { commerce, userId, token };
}

test.describe('Paid Launch Flow', () => {
  test('launch page loads with presets and pricing', async ({ page }) => {
    await navigateToLaunch(page);

    // Page title
    await expect(
      page.getByRole('heading', { name: 'Launch a Bot' }),
    ).toBeVisible({ timeout: 15_000 });

    // All four preset cards are visible
    for (const name of PRESET_NAMES) {
      await expect(page.getByRole('heading', { name, exact: true })).toBeVisible({ timeout: 15_000 });
    }

    // Pricing badges show $/hr format (e.g. "$0.02/hr")
    const priceBadges = page.locator('text=/\\$\\d+\\.\\d{2}\\/hr/');
    await expect(priceBadges.first()).toBeVisible({ timeout: 15_000 });
    const badgeCount = await priceBadges.count();
    expect(badgeCount).toBeGreaterThanOrEqual(PRESET_NAMES.length);

    // "Connect your own machine" section
    await expect(
      page.getByRole('heading', { name: 'Connect your own machine' }),
    ).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('npx @hanzo/bot')).toBeVisible();
  });

  test('shows balance when user has credits', async ({ page }) => {
    await navigateToLaunch(page);

    const { commerce, userId } = await resolveCommerceContext(page);
    if (!commerce || !userId) {
      test.skip(true, 'No auth token available — cannot verify balance');
      return;
    }

    let balance: Awaited<ReturnType<CommerceHelper['getBalance']>> | null = null;
    try {
      balance = await commerce.getBalance(userId);
    } catch {
      test.skip(true, 'Commerce API unreachable — cannot verify balance');
      return;
    }

    if (balance.available <= 0) {
      test.skip(true, 'User has no credits — cannot verify balance display');
      return;
    }

    // The balance display should be visible with a dollar amount
    const balanceText = page.getByText(/^Balance:\s*\$/);
    await expect(balanceText).toBeVisible({ timeout: 15_000 });
  });

  test('disables launch for unaffordable presets', async ({ page }) => {
    await navigateToLaunch(page);

    const { commerce, userId } = await resolveCommerceContext(page);
    if (!commerce || !userId) {
      test.skip(true, 'No auth token available — cannot verify affordability');
      return;
    }

    let balance: Awaited<ReturnType<CommerceHelper['getBalance']>> | null = null;
    try {
      balance = await commerce.getBalance(userId);
    } catch {
      test.skip(true, 'Commerce API unreachable');
      return;
    }

    // The page needs the balance/presets API to have returned affordability data.
    // If the user can afford every preset we cannot verify the disabled state.
    // GPU costs 7 cents/hr — if user has >= 7 cents available, all presets are affordable.
    if (balance.available >= 7) {
      test.skip(true, 'User can afford all presets — cannot verify disabled state');
      return;
    }

    if (balance.available <= 0) {
      // All buttons should show "Insufficient Funds" and be disabled
      const insufficientButtons = page.getByRole('button', { name: /Insufficient Funds/i });
      await expect(insufficientButtons.first()).toBeVisible({ timeout: 15_000 });
      const count = await insufficientButtons.count();
      expect(count).toBe(PRESET_NAMES.length);

      for (let i = 0; i < count; i++) {
        await expect(insufficientButtons.nth(i)).toBeDisabled();
      }
    } else {
      // Some presets affordable, some not — check that unaffordable ones are disabled.
      // Cards with opacity-60 class indicate unaffordable presets.
      const disabledCards = page.locator('.opacity-60');
      const disabledCount = await disabledCards.count();
      expect(disabledCount).toBeGreaterThanOrEqual(1);
    }
  });

  test('shows billing error on 402 from provision attempt', async ({ page }) => {
    await navigateToLaunch(page);

    const { commerce, userId } = await resolveCommerceContext(page);
    if (!commerce || !userId) {
      test.skip(true, 'No auth token available — cannot verify billing error');
      return;
    }

    let balance: Awaited<ReturnType<CommerceHelper['getBalance']>> | null = null;
    try {
      balance = await commerce.getBalance(userId);
    } catch {
      test.skip(true, 'Commerce API unreachable');
      return;
    }

    if (balance.available >= STARTER_CENTS_PER_HOUR) {
      test.skip(true, 'User has enough funds — cannot trigger 402 billing error');
      return;
    }

    // Click the first Launch button (which may show "Insufficient Funds" for
    // balance-aware clients, or "Launch" if balance API was slow).
    // Either way, the server will return 402 for a zero-balance user.
    const launchButtons = page.getByRole('button', { name: /Launch|Insufficient Funds/i });
    await expect(launchButtons.first()).toBeVisible({ timeout: 15_000 });

    // If the button is disabled (client already knows funds are insufficient),
    // we intercept the provision call to simulate the 402 so the error alert renders.
    const firstButton = launchButtons.first();
    const isDisabled = await firstButton.isDisabled();

    if (isDisabled) {
      // Enable the button temporarily via route interception: let the click go through
      // and mock a 402 response so the billing error UI renders.
      await page.route('**/cloud/nodes/provision*', (route) =>
        route.fulfill({
          status: 402,
          contentType: 'application/json',
          body: JSON.stringify({
            error: 'insufficient_funds',
            balance_cents: balance!.available,
            required_cents: STARTER_CENTS_PER_HOUR,
          }),
        }),
      );
      // Remove the disabled attribute so we can click
      await firstButton.evaluate((el) => el.removeAttribute('disabled'));
    }

    await firstButton.click();

    // Wait for the amber billing error alert
    const billingAlert = page.locator('.border-amber-500\\/50');
    await expect(billingAlert).toBeVisible({ timeout: 15_000 });

    // Verify error message content
    await expect(page.getByText('Insufficient funds to launch this bot.')).toBeVisible();
    await expect(page.getByText(/Required:/)).toBeVisible();
    await expect(page.getByText(/Your balance:/)).toBeVisible();

    // "Add Funds" link should point to billing.hanzo.ai
    const addFundsLink = page.getByRole('link', { name: 'Add Funds' });
    await expect(addFundsLink).toBeVisible();
    await expect(addFundsLink).toHaveAttribute('href', 'https://billing.hanzo.ai');
  });

  test('successful provision redirects to nodes page', async ({ page }) => {
    await navigateToLaunch(page);

    const { commerce, userId } = await resolveCommerceContext(page);
    if (!commerce || !userId) {
      test.skip(true, 'No auth token available — cannot verify provisioning');
      return;
    }

    let balance: Awaited<ReturnType<CommerceHelper['getBalance']>> | null = null;
    try {
      balance = await commerce.getBalance(userId);
    } catch {
      test.skip(true, 'Commerce API unreachable');
      return;
    }

    // Ensure the user has at least enough for the Starter preset
    if (balance.available < STARTER_CENTS_PER_HOUR) {
      try {
        await commerce.grantStarterCredit(userId);
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err);
        test.skip(true, `Cannot grant starter credit: ${msg}`);
        return;
      }

      // Re-check balance after grant
      try {
        balance = await commerce.getBalance(userId);
      } catch {
        test.skip(true, 'Commerce API unreachable after credit grant');
        return;
      }

      if (balance.available < STARTER_CENTS_PER_HOUR) {
        test.skip(true, 'Balance still insufficient after starter credit grant');
        return;
      }

      // Reload so the page picks up the new balance
      await navigateToLaunch(page);
    }

    // Locate the Starter card and its Launch button
    const starterCard = page.locator('div.rounded-xl').filter({ hasText: 'Starter' }).first();
    await expect(starterCard).toBeVisible({ timeout: 15_000 });

    const starterLaunchButton = starterCard.getByRole('button', { name: 'Launch' });

    await expect(starterLaunchButton).toBeVisible({ timeout: 15_000 });
    await expect(starterLaunchButton).toBeEnabled();
    await starterLaunchButton.click();

    // Wait for redirect to /nodes
    await page.waitForURL(/\/nodes/, { timeout: 30_000 });

    // Verify we landed on the My Bots page
    await expect(
      page.getByRole('heading', { name: /My Bots/i }),
    ).toBeVisible({ timeout: 15_000 });
  });

  test('new user sees presets with correct specs', async ({ page }) => {
    await navigateToLaunch(page);

    // Wait for presets to render
    await expect(
      page.getByRole('heading', { name: 'Starter', exact: true }),
    ).toBeVisible({ timeout: 15_000 });

    for (const [name, specs] of Object.entries(PRESET_SPECS)) {
      const card = page.locator('div.rounded-xl').filter({ hasText: name }).first();
      await expect(card).toBeVisible({ timeout: 15_000 });

      // Verify vCPU count
      const vcpuText = specs.vcpus === 1 ? '1 vCPU' : `${specs.vcpus} vCPUs`;
      await expect(card.getByText(vcpuText)).toBeVisible();

      // Verify RAM
      await expect(card.getByText(`${specs.memoryGB}GB RAM`)).toBeVisible();

      // Verify price badge ($/hr format)
      await expect(card.locator('text=/\\$\\d+\\.\\d{2}\\/hr/')).toBeVisible();
    }
  });
});
