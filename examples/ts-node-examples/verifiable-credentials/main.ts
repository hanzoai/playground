import 'dotenv/config';
import { Bot } from '@playground/sdk';
import { botsRouter } from './bots.js';

/**
 * Verifiable Credentials Example
 *
 * This example demonstrates how to use DID (Decentralized Identifiers) and
 * Verifiable Credentials (VCs) in Playground to create cryptographically
 * verifiable audit trails for agent executions.
 *
 * Each bot in this example generates a VC that:
 * - Records input/output hashes for tamper detection
 * - Links to caller and target DIDs for accountability
 * - Is signed by the control plane's issuer DID
 * - Can be verified independently for compliance/audit
 *
 * Prerequisites:
 * 1. Control plane running with DID enabled (features.did.enabled: true)
 * 2. Keystore directory exists (./data/keys on control plane)
 *
 * Usage:
 *   pnpm dev:vc
 *
 * Then test with:
 *   curl -X POST http://localhost:8080/api/v1/execute/vc-demo.vc_process \
 *     -H "Content-Type: application/json" \
 *     -d '{"input": {"text": "Hello, Verifiable World!"}}'
 */

async function main() {
  const bot = new Bot({
    nodeId: process.env.HANZO_NODE_ID ?? 'vc-demo',
    playgroundUrl: process.env.PLAYGROUND_URL ?? 'http://localhost:8080',
    port: Number(process.env.PORT ?? 8006),
    version: '1.0.0',
    devMode: true,

    // DID/VC is enabled by default, but we explicitly set it here for clarity
    didEnabled: true,

    // Optional: AI config for the AI-powered bot
    aiConfig: {
      provider: 'openai',
      model: 'gpt-4o-mini',
      apiKey: process.env.OPENAI_API_KEY,
    },
  });

  // Include the VC-enabled bots
  bot.includeRouter(botsRouter);

  await bot.serve();

  console.log(`
╔════════════════════════════════════════════════════════════════════╗
║           Verifiable Credentials Demo Bot Started                  ║
╠════════════════════════════════════════════════════════════════════╣
║  Bot ID:       ${bot.config.nodeId.padEnd(50)}║
║  Port:         ${String(bot.config.port).padEnd(50)}║
║  DID Enabled:  ${String(bot.config.didEnabled).padEnd(50)}║
╠════════════════════════════════════════════════════════════════════╣
║  Available Bots:                                              ║
║  • vc_process      - Basic processing with VC generation           ║
║  • vc_analyze      - AI analysis with VC audit trail               ║
║  • vc_transform    - Data transformation with VC proof             ║
║  • vc_chain        - Multi-step workflow with chained VCs          ║
╠════════════════════════════════════════════════════════════════════╣
║  Test Commands:                                                    ║
║                                                                    ║
║  # Basic VC generation:                                            ║
║  curl -X POST http://localhost:8080/api/v1/execute/vc-demo.vc_process \\
║    -H "Content-Type: application/json" \\                          ║
║    -d '{"input": {"text": "Hello World"}}'                         ║
║                                                                    ║
║  # Check workflow in UI for VC badge (green checkmark)             ║
╚════════════════════════════════════════════════════════════════════╝
`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    console.error('Failed to start bot:', err);
    process.exit(1);
  });
}
