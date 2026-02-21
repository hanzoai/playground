import { type Page, type Locator, expect } from '@playwright/test';

/**
 * Page object for the AuthGuard login screen and the hanzo.id Casdoor login form.
 */
export class LoginPage {
  readonly page: Page;

  // AuthGuard elements (app.hanzo.bot)
  readonly signInWithHanzoButton: Locator;
  readonly useApiKeyLink: Locator;
  readonly apiKeyInput: Locator;
  readonly connectWithApiKeyButton: Locator;
  readonly errorMessage: Locator;
  readonly logo: Locator;

  // Casdoor form elements (hanzo.id)
  readonly casdoorEmailInput: Locator;
  readonly casdoorPasswordInput: Locator;
  readonly casdoorSubmitButton: Locator;
  readonly casdoorSignupLink: Locator;

  constructor(page: Page) {
    this.page = page;

    // AuthGuard (app side)
    this.signInWithHanzoButton = page.getByRole('button', { name: /sign in with hanzo/i });
    this.useApiKeyLink = page.getByText(/use api key instead/i);
    this.apiKeyInput = page.locator('input[placeholder="API Key"]');
    this.connectWithApiKeyButton = page.getByRole('button', { name: /connect with api key/i });
    this.errorMessage = page.locator('.text-destructive');
    this.logo = page.locator('text=Hanzo Playground');

    // Casdoor form (hanzo.id side)
    this.casdoorEmailInput = page.locator('input[name="username"]')
      .or(page.locator('input[name="email"]'))
      .or(page.locator('input[type="email"]'))
      .or(page.locator('input[placeholder*="email" i]'))
      .or(page.locator('input[placeholder*="username" i]'));

    this.casdoorPasswordInput = page.locator('input[name="password"]')
      .or(page.locator('input[type="password"]'));

    this.casdoorSubmitButton = page.getByRole('button', { name: /sign in|log in|login|submit/i })
      .or(page.locator('button[type="submit"]'));

    this.casdoorSignupLink = page.getByText(/sign up|register|create account/i);
  }

  async goto() {
    await this.page.goto('/', { waitUntil: 'networkidle' });
  }

  async expectAuthGuardVisible() {
    await expect(this.signInWithHanzoButton).toBeVisible({ timeout: 15_000 });
  }

  async clickSignInWithHanzo() {
    await this.signInWithHanzoButton.click();
  }

  async waitForCasdoorPage() {
    const iamUrl = process.env.E2E_IAM_SERVER_URL || 'https://hanzo.id';
    await this.page.waitForURL(`${iamUrl}/**`, { timeout: 30_000 });
  }

  async fillCasdoorCredentials(email: string, password: string) {
    await this.casdoorEmailInput.first().fill(email, { timeout: 15_000 });
    await this.casdoorPasswordInput.first().fill(password);
  }

  async submitCasdoorForm() {
    await this.casdoorSubmitButton.first().click();
  }

  async waitForAuthCallback() {
    const baseURL = process.env.E2E_BASE_URL || 'https://app.hanzo.bot';
    await this.page.waitForURL(`${baseURL}/**`, { timeout: 30_000 });
  }

  async waitForDashboard() {
    await this.page.waitForURL(/\/(dashboard|bots|nodes|executions|workflows|canvas|spaces)/, {
      timeout: 30_000,
    });
  }

  /**
   * Full login flow: click sign in → fill Casdoor form → wait for dashboard.
   */
  async performFullLogin(email: string, password: string) {
    await this.expectAuthGuardVisible();
    await this.clickSignInWithHanzo();
    await this.waitForCasdoorPage();
    await this.fillCasdoorCredentials(email, password);
    await this.submitCasdoorForm();
    await this.waitForAuthCallback();
    await this.waitForDashboard();
  }
}
