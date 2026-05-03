/**
 * Global setup — runs once before all projects.
 * Validates required env vars and checks that the target app is alive.
 */
async function globalSetup() {
  const required = [
    'E2E_BASE_URL',
    'E2E_IAM_USER_EMAIL',
    'E2E_IAM_USER_PASSWORD',
    'E2E_IAM_SERVER_URL',
    'E2E_IAM_CLIENT_ID',
  ];

  const missing = required.filter((k) => !process.env[k]);
  if (missing.length > 0) {
    throw new Error(
      `Missing required E2E env vars: ${missing.join(', ')}\n` +
        'Copy .env.e2e.example to .env.e2e and fill in values.'
    );
  }

  // Health check — app must be reachable
  const baseURL = process.env.E2E_BASE_URL!;
  try {
    const healthURL = process.env.E2E_API_BASE_URL
      ? `${process.env.E2E_API_BASE_URL}/health`
      : 'https://api.hanzo.bot/v1/health';
    const res = await fetch(healthURL, { signal: AbortSignal.timeout(10_000) });
    if (!res.ok) {
      console.warn(`Health check returned ${res.status} — tests may fail`);
    } else {
      console.log(`Health check OK: ${healthURL} → ${res.status}`);
    }
  } catch (err) {
    throw new Error(`Cannot reach API health endpoint — is the app running?\n${err}`);
  }

  // IAM server health check
  const iamURL = process.env.E2E_IAM_SERVER_URL!;
  try {
    const res = await fetch(iamURL, { signal: AbortSignal.timeout(10_000) });
    console.log(`IAM server reachable: ${iamURL} → ${res.status}`);
  } catch (err) {
    throw new Error(`Cannot reach IAM server ${iamURL}\n${err}`);
  }
}

export default globalSetup;
