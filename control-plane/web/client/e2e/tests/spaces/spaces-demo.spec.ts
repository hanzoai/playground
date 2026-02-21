/**
 * Spaces Demo — walks through all new features with mocked API responses.
 *
 * Run:  npx playwright test --config=e2e/tests/spaces/demo.config.ts --headed
 */

import { test, expect, type Page } from '@playwright/test';

test.use({
  baseURL: 'http://localhost:5173',
  storageState: undefined as any,
});

// --- Mock data ---

const SPACES = [
  {
    id: 'space-001',
    org_id: 'org-hanzo',
    name: 'Default',
    slug: 'default',
    description: 'Auto-created workspace',
    created_by: 'user-001',
    created_at: '2026-02-01T00:00:00Z',
    updated_at: '2026-02-18T00:00:00Z',
  },
  {
    id: 'space-002',
    org_id: 'org-hanzo',
    name: 'Marketing AI',
    slug: 'marketing-ai',
    description: 'Shared workspace for marketing team',
    created_by: 'user-002',
    created_at: '2026-02-10T00:00:00Z',
    updated_at: '2026-02-18T00:00:00Z',
  },
];

const MEMBERS_001 = [
  { space_id: 'space-001', user_id: 'alice@hanzo.ai', role: 'owner', created_at: '2026-02-01T00:00:00Z' },
];

const MEMBERS_002 = [
  { space_id: 'space-002', user_id: 'bob@hanzo.ai', role: 'owner', created_at: '2026-02-10T00:00:00Z' },
  { space_id: 'space-002', user_id: 'carol@hanzo.ai', role: 'admin', created_at: '2026-02-11T00:00:00Z' },
  { space_id: 'space-002', user_id: 'dave@hanzo.ai', role: 'member', created_at: '2026-02-12T00:00:00Z' },
];

const NODES = [
  {
    space_id: 'space-001',
    node_id: 'node-local-1',
    name: 'Dev Laptop',
    type: 'local',
    endpoint: 'http://localhost:9090',
    status: 'online',
    os: 'macOS',
    registered_at: '2026-02-01T00:00:00Z',
    last_seen: '2026-02-18T12:00:00Z',
  },
];

const BOTS = [
  {
    space_id: 'space-001',
    bot_id: 'bot-001',
    node_id: 'node-local-1',
    agent_id: 'agent-001',
    name: 'Code Assistant',
    model: 'claude-sonnet-4-20250514',
    view: 'chat',
    status: 'running',
  },
];

async function setupMocks(page: Page) {
  // Use a single route handler that dispatches by URL + method
  await page.route('**/api/v1/spaces**', async (route) => {
    const url = route.request().url();
    const method = route.request().method();

    // POST /spaces — create
    if (url.endsWith('/spaces') && method === 'POST') {
      const body = route.request().postDataJSON();
      const newSpace = {
        id: `space-${Date.now()}`,
        org_id: 'org-hanzo',
        name: body.name,
        slug: body.name.toLowerCase().replace(/\s+/g, '-'),
        description: body.description || '',
        created_by: 'user-001',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      SPACES.push(newSpace);
      return route.fulfill({ json: newSpace });
    }

    // GET /spaces — list
    if (url.endsWith('/spaces') && method === 'GET') {
      return route.fulfill({ json: { spaces: SPACES } });
    }

    // Members
    if (url.includes('/space-001/members')) {
      if (method === 'POST') {
        const body = route.request().postDataJSON();
        const newMember = {
          space_id: 'space-001',
          user_id: body.user_id,
          role: body.role,
          created_at: new Date().toISOString(),
        };
        MEMBERS_001.push(newMember);
        return route.fulfill({ json: newMember });
      }
      return route.fulfill({ json: { members: MEMBERS_001 } });
    }
    if (url.includes('/space-002/members')) {
      return route.fulfill({ json: { members: MEMBERS_002 } });
    }

    // Nodes
    if (url.includes('/space-001/nodes')) {
      return route.fulfill({ json: { nodes: NODES } });
    }
    if (url.includes('/nodes')) {
      return route.fulfill({ json: { nodes: [] } });
    }

    // Bots
    if (url.includes('/space-001/bots')) {
      return route.fulfill({ json: { bots: BOTS } });
    }
    if (url.includes('/bots')) {
      return route.fulfill({ json: { bots: [] } });
    }

    // Individual space GET
    if (url.includes('/space-001')) {
      return route.fulfill({ json: SPACES[0] });
    }
    if (url.includes('/space-002')) {
      return route.fulfill({ json: SPACES[1] });
    }

    // Fallback
    return route.fulfill({ json: {} });
  });
}

test.describe('Spaces Features Demo (mocked)', () => {

  test('Full walkthrough', async ({ page }) => {
    // Log console errors for debugging
    page.on('console', (msg) => {
      if (msg.type() === 'error') console.log('BROWSER ERROR:', msg.text());
    });
    page.on('pageerror', (err) => console.log('PAGE ERROR:', err.message));

    await setupMocks(page);

    // Set space-001 as active so the canvas + settings pages have data
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.setItem('hanzo-space-active', 'space-001');
    });

    // --- 1. Playground / Canvas Page ---
    console.log('\n=== Phase 1: Playground (Canvas) ===');
    await page.goto('/playground');
    await page.waitForTimeout(2500);
    await page.screenshot({ path: 'test-results/01-playground.png', fullPage: true });
    console.log('✓ Playground loaded with active space "Default"');

    // --- 2. Spaces Page with badges ---
    console.log('\n=== Phase 2: Spaces Page ===');
    await page.goto('/spaces');
    await page.waitForTimeout(2500);

    // Should show the heading
    await expect(page.locator('h1:has-text("Spaces")')).toBeVisible();

    // Should show space cards
    const defaultCard = page.locator('text=Default').first();
    await expect(defaultCard).toBeVisible();
    console.log('✓ "Default" space card visible');

    const marketingCard = page.locator('text=Marketing AI').first();
    await expect(marketingCard).toBeVisible();
    console.log('✓ "Marketing AI" space card visible');

    // Check for Active badge on space-001
    const activeBadge = page.locator('span:has-text("Active")');
    await expect(activeBadge).toBeVisible();
    console.log('✓ "Active" badge visible on Default space');

    // Check for Shared badge on space-002 (3 members)
    const sharedBadge = page.locator('text=/Shared \\(3\\)/');
    await expect(sharedBadge).toBeVisible();
    console.log('✓ "Shared (3)" badge visible on Marketing AI space');

    await page.screenshot({ path: 'test-results/02-spaces-with-badges.png', fullPage: true });

    // --- 3. Space Settings Page ---
    console.log('\n=== Phase 3: Space Settings ===');
    await page.goto('/spaces/settings');
    await page.waitForTimeout(2500);

    // Space name should be visible
    await expect(page.locator('h1:has-text("Default")')).toBeVisible();
    console.log('✓ Space settings loaded for "Default"');

    // Nodes section
    await expect(page.locator('h2:has-text("Nodes")')).toBeVisible();
    await expect(page.locator('text=Dev Laptop')).toBeVisible();
    console.log('✓ Nodes section with "Dev Laptop" node');

    // Bots section
    await expect(page.locator('h2:has-text("Bots")')).toBeVisible();
    await expect(page.locator('text=Code Assistant')).toBeVisible();
    console.log('✓ Bots section with "Code Assistant" bot');

    // Members section
    await expect(page.locator('h2:has-text("Members")')).toBeVisible();
    await expect(page.locator('text=alice@hanzo.ai')).toBeVisible();
    console.log('✓ Members section with "alice@hanzo.ai"');

    // Connected Platform section
    await expect(page.locator('h2:has-text("Connected Platform")')).toBeVisible();
    console.log('✓ Connected Platform section visible');

    // Details section
    await expect(page.locator('h2:has-text("Details")')).toBeVisible();
    console.log('✓ Details section visible');

    await page.screenshot({ path: 'test-results/03-space-settings-full.png', fullPage: true });

    // --- 4. Add a member ---
    console.log('\n=== Phase 4: Add Member ===');
    const userIdInput = page.locator('#member-user-id');
    await userIdInput.fill('eve@hanzo.ai');

    const roleSelect = page.locator('#member-role');
    await roleSelect.selectOption('admin');

    await page.screenshot({ path: 'test-results/04-add-member-form.png', fullPage: true });

    await page.locator('button:has-text("Add")').click();
    await page.waitForTimeout(1000);

    // New member should appear
    await expect(page.locator('text=eve@hanzo.ai')).toBeVisible();
    console.log('✓ Added member "eve@hanzo.ai" as admin');
    await page.screenshot({ path: 'test-results/05-member-added.png', fullPage: true });

    // --- 5. Connected Platform form ---
    console.log('\n=== Phase 5: Connect Team Platform ===');
    const accountUrlInput = page.locator('#team-account-url');
    await accountUrlInput.fill('https://team.hanzo.ai/api/account');

    const tokenInput = page.locator('#team-token');
    await tokenInput.fill('hztp_live_abc123');

    await page.screenshot({ path: 'test-results/06-platform-connect-form.png', fullPage: true });
    console.log('✓ Platform connection form filled');

    // --- 6. Create new space flow ---
    console.log('\n=== Phase 6: Create New Space ===');
    await page.goto('/spaces');
    await page.waitForTimeout(1500);

    await page.locator('button:has-text("New Space")').click();
    await page.waitForTimeout(300);

    await page.locator('input[placeholder="Space name"]').fill('Backend Team');
    await page.locator('input[placeholder*="Description"]').fill('Shared workspace for backend engineers');

    await page.screenshot({ path: 'test-results/07-create-new-space.png', fullPage: true });
    console.log('✓ Create space form filled with "Backend Team"');

    console.log('\n=== All phases verified! ===\n');
  });

});
