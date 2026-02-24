import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page object for top navigation, sidebar, and OrgProjectSwitcher.
 */
export class TopNavigationPage {
  readonly page: Page;

  // Top nav
  readonly breadcrumb: Locator;
  readonly themeToggle: Locator;
  readonly orgProjectSwitcher: Locator;

  // Sidebar
  readonly sidebarTrigger: Locator;
  readonly sidebarLogo: Locator;
  readonly sidebarVersion: Locator;

  // Sidebar nav links
  readonly dashboardLink: Locator;
  readonly botsLink: Locator;
  readonly nodesLink: Locator;
  readonly executionsLink: Locator;
  readonly workflowsLink: Locator;
  readonly spacesLink: Locator;
  readonly playgroundLink: Locator;
  readonly teamsLink: Locator;
  readonly didExplorerLink: Locator;
  readonly credentialsLink: Locator;
  readonly settingsLink: Locator;

  // User menu
  readonly userMenuButton: Locator;
  readonly signOutButton: Locator;

  constructor(page: Page) {
    this.page = page;

    // Top nav
    this.breadcrumb = page.locator('nav[aria-label="breadcrumb"]').or(page.locator('[class*="breadcrumb"]'));
    this.themeToggle = page.getByRole('button', { name: /toggle theme|dark|light/i });
    this.orgProjectSwitcher = page.locator('[class*="OrgProjectSwitcher"], [class*="org-project"]')
      .or(page.getByText(/organization|org/i).first());

    // Sidebar
    this.sidebarTrigger = page.locator('[class*="SidebarTrigger"]')
      .or(page.getByRole('button', { name: /toggle sidebar|menu/i }));
    this.sidebarLogo = page.getByText(/hanzo bot/i).first();
    this.sidebarVersion = page.getByText(/v\d+\.\d+\.\d+/);

    // Sidebar nav links — use href to avoid matching breadcrumb/heading duplicates
    this.dashboardLink = page.locator('a[href="/dashboard"]').first();
    this.botsLink = page.locator('a[href="/bots/all"]').first();
    this.nodesLink = page.locator('a[href="/nodes"]').first();
    this.executionsLink = page.locator('a[href="/executions"]').first();
    this.workflowsLink = page.locator('a[href="/workflows"]').first();
    this.spacesLink = page.locator('a[href="/spaces"]').first();
    this.playgroundLink = page.locator('a[href="/playground"]').first();
    this.teamsLink = page.locator('a[href="/teams"]').first();
    this.didExplorerLink = page.locator('a[href="/identity/dids"]').first();
    this.credentialsLink = page.locator('a[href="/identity/credentials"]').first();
    this.settingsLink = page.locator('a[href="/settings"]').first();

    // User menu
    this.userMenuButton = page.locator('[class*="sidebar-footer"] button, [class*="user-menu"]')
      .or(page.getByRole('button').filter({ hasText: /account|user|profile/i }));
    this.signOutButton = page.getByText(/sign out/i);
  }

  // ---- Navigation helpers ----

  async navigateTo(link: Locator, expectedPath: RegExp) {
    await link.click();
    await this.page.waitForURL(expectedPath, { timeout: 15_000 });
  }

  async goToDashboard() {
    await this.navigateTo(this.dashboardLink, /\/dashboard/);
  }

  async goToBots() {
    await this.navigateTo(this.botsLink, /\/bots\/all/);
  }

  async goToNodes() {
    await this.navigateTo(this.nodesLink, /\/nodes/);
  }

  async goToExecutions() {
    await this.navigateTo(this.executionsLink, /\/executions/);
  }

  async goToWorkflows() {
    await this.navigateTo(this.workflowsLink, /\/workflows/);
  }

  // ---- OrgProjectSwitcher ----

  async expectOrgSwitcherVisible() {
    // The OrgProjectSwitcher renders org/project dropdowns in IAM mode
    await expect(this.orgProjectSwitcher).toBeVisible({ timeout: 10_000 });
  }

  async getOrgSwitcherText(): Promise<string> {
    return (await this.orgProjectSwitcher.textContent()) || '';
  }

  // ---- User menu ----

  async openUserMenu() {
    await this.userMenuButton.first().click();
  }

  async signOut() {
    await this.openUserMenu();
    await this.signOutButton.click();
  }

  // ---- Breadcrumb ----

  async expectBreadcrumbContains(text: string) {
    await expect(this.breadcrumb).toContainText(text, { timeout: 5_000 });
  }

  // ---- Sidebar state ----

  async expectSidebarVisible() {
    await expect(this.sidebarLogo).toBeVisible({ timeout: 5_000 });
  }

  async toggleSidebar() {
    await this.sidebarTrigger.first().click();
  }
}
