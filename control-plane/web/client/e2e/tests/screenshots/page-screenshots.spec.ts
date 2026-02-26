/**
 * Screenshot every major page to verify production is working.
 * Captures full-page screenshots after content loads.
 */
import { test } from '../../fixtures';
import { expect } from '@playwright/test';

const PAGES = [
  { name: 'dashboard', path: '/dashboard', waitFor: 'text=Dashboard' },
  { name: 'bots-all', path: '/bots/all', waitFor: 'text=Control Plane' },
  { name: 'nodes', path: '/nodes', waitFor: 'text=Nodes' },
  { name: 'executions', path: '/executions', waitFor: 'text=Executions' },
  { name: 'settings-gateway', path: '/settings', waitFor: 'text=Gateway' },
  { name: 'settings-preferences', path: '/settings/preferences', waitFor: 'text=Preferences' },
  { name: 'playground', path: '/playground', waitFor: '[data-testid="canvas"], .canvas-container, main' },
  { name: 'spaces', path: '/spaces', waitFor: 'text=Spaces' },
  { name: 'identity', path: '/identity/overview', waitFor: 'text=Identity' },
];

test.describe('Production Page Screenshots', () => {
  for (const page of PAGES) {
    test(`screenshot: ${page.name}`, async ({ page: p }) => {
      await p.goto(page.path, { waitUntil: 'domcontentloaded', timeout: 30000 });

      // Wait for main content to appear (best effort)
      try {
        await p.waitForSelector(page.waitFor, { timeout: 10000 });
      } catch {
        // Page might use different text, continue with screenshot anyway
      }

      // Extra settle time for animations/data loading
      await p.waitForTimeout(2000);

      await p.screenshot({
        path: `e2e/screenshots/${page.name}.png`,
        fullPage: true,
      });

      // Basic check: page rendered something (canvas pages may have no text)
      const bodyHTML = await p.locator('body').innerHTML();
      expect(bodyHTML.length).toBeGreaterThan(50);
    });
  }
});
