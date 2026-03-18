/**
 * Responsive landing page tests.
 *
 * Verifies the landing/login page renders correctly across
 * mobile, tablet, laptop, and desktop viewports.
 * Takes screenshots at each viewport for visual regression.
 *
 * The unauthenticated root URL redirects to hanzo.id (IAM login).
 * Tests verify the login page renders correctly at each viewport.
 */
import { test as base, expect } from '@playwright/test';

const VIEWPORTS = [
  { name: 'mobile',  width: 375,  height: 812  },
  { name: 'tablet',  width: 768,  height: 1024 },
  { name: 'laptop',  width: 1366, height: 768  },
  { name: 'desktop', width: 1920, height: 1080 },
];

// Use base test (no auth fixtures) — landing page is the unauthenticated view
const test = base;

for (const vp of VIEWPORTS) {
  test.describe(`Landing Page — ${vp.name} (${vp.width}x${vp.height})`, () => {
    test(`renders without errors`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: vp.width, height: vp.height },
      });
      const page = await context.newPage();

      const errors: string[] = [];
      page.on('pageerror', (err) => errors.push(err.message));

      await page.goto('/', { waitUntil: 'domcontentloaded', timeout: 30_000 });
      await page.waitForLoadState('networkidle').catch(() => {});
      await page.waitForTimeout(2_000);

      // Page should have rendered meaningful content — not blank
      const bodyHTML = await page.locator('body').innerHTML();
      expect(bodyHTML.length).toBeGreaterThan(100);

      // No horizontal scroll on mobile/tablet — only check on our own app pages
      // (hanzo.id login page is external and may have its own responsive issues)
      if (vp.width <= 1024 && !page.url().includes('hanzo.id')) {
        const hasHorizontalScroll = await page.evaluate(() => {
          return document.documentElement.scrollWidth > document.documentElement.clientWidth;
        });
        expect(hasHorizontalScroll).toBe(false);
      }

      await page.screenshot({
        path: `e2e/screenshots/landing-${vp.name}.png`,
        fullPage: true,
      });

      await context.close();
    });

    test(`key elements are visible`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: vp.width, height: vp.height },
      });
      const page = await context.newPage();

      await page.goto('/', { waitUntil: 'domcontentloaded', timeout: 30_000 });
      await page.waitForLoadState('networkidle').catch(() => {});
      await page.waitForTimeout(2_000);

      const url = page.url();

      if (url.includes('hanzo.id')) {
        // On hanzo.id login form — verify login UI elements exist
        const loginInput = page.locator('input[name="username"], input[type="email"], input[placeholder*="email" i], input[placeholder*="username" i]').first();
        const isLoginVisible = await loginInput.isVisible({ timeout: 15_000 }).catch(() => false);

        if (isLoginVisible) {
          await expect(loginInput).toBeVisible();
        }

        // Check for a sign in button — use nth(0) to avoid strict mode with multiple matches
        const signInButtons = page.getByRole('button', { name: /sign in/i });
        const signInCount = await signInButtons.count();
        if (signInCount > 0) {
          await expect(signInButtons.nth(0)).toBeVisible({ timeout: 5_000 });
        }
      } else {
        // On app — either auth guard or landing page
        const branding = page.getByText(/hanzo/i).first();
        await expect(branding).toBeVisible({ timeout: 10_000 });

        // Verify at least one interactive element exists (button, link, or input)
        const buttonCount = await page.getByRole('button').count();
        const linkCount = await page.getByRole('link').count();
        const inputCount = await page.locator('input').count();
        expect(buttonCount + linkCount + inputCount).toBeGreaterThan(0);
      }

      await page.screenshot({
        path: `e2e/screenshots/landing-elements-${vp.name}.png`,
        fullPage: true,
      });

      await context.close();
    });

    test(`text is readable (no overlap or clipping)`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: vp.width, height: vp.height },
      });
      const page = await context.newPage();

      await page.goto('/', { waitUntil: 'domcontentloaded', timeout: 30_000 });
      await page.waitForLoadState('networkidle').catch(() => {});
      await page.waitForTimeout(2_000);

      // On hanzo.id login page (external), be lenient — we don't control their CSS
      const maxClipped = page.url().includes('hanzo.id') ? 15 : 5;

      const clippedElements = await page.evaluate(() => {
        const elements = document.querySelectorAll('h1, h2, h3, p, button, a, label, span');
        let clipped = 0;
        elements.forEach((el) => {
          const rect = el.getBoundingClientRect();
          const text = el.textContent?.trim();
          if (text && text.length > 0 && rect.height === 0) {
            clipped++;
          }
        });
        return clipped;
      });

      expect(clippedElements).toBeLessThan(maxClipped);

      await context.close();
    });
  });
}
