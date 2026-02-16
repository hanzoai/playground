import { Bot } from '@playground/sdk';
import dotenv from 'dotenv';

dotenv.config();

const bot = new Bot({
  nodeId: process.env.HANZO_NODE_ID ?? 'ts-serverless-hello',
  version: '1.0.0',
  deploymentType: 'serverless',
  playgroundUrl: process.env.PLAYGROUND_URL ?? 'http://localhost:8080',
  devMode: true
});

bot.bot('hello', async (ctx) => ({
  greeting: `Hello, ${ctx.input.name ?? 'Playground'}!`,
  runId: ctx.runId,
  executionId: ctx.executionId
}));

bot.bot('relay', async (ctx) => {
  const target = (process.env.CHILD_TARGET ?? ctx.input.target) as string | undefined;
  if (!target) {
    return { error: 'target is required' };
  }

  const downstream = await bot.call(target, { message: ctx.input.message ?? 'ping' });
  return { target, downstream };
});

// Exported handler works for AWS Lambda/Cloud Functions and raw HTTP (Vercel/Netlify).
// You can adapt any platform-specific event shape here (e.g., Supabase/Netlify payloads).
export const handler = bot.handler((event) => {
  const body = typeof event?.body === 'string' ? safeJson(event.body) : event?.body;
  const input = event?.input ?? body?.input ?? body ?? {};
  return {
    path: event?.rawPath || event?.path || '/execute',
    headers: event?.headers ?? {},
    queryStringParameters: event?.queryStringParameters ?? event?.query ?? {},
    bot: event?.bot ?? event?.target,
    target: event?.target ?? event?.bot,
    input,
    executionContext: event?.executionContext ?? event?.execution_context
  };
});

function safeJson(value: string | undefined) {
  if (!value) return {};
  try {
    return JSON.parse(value);
  } catch {
    return {};
  }
}

// Optional local runner for smoke-testing without heartbeats.
if (import.meta.url === `file://${process.argv[1]}`) {
  const port = Number(process.env.PORT ?? 8787);
  const { default: express } = await import('express');

  const app = express();
  app.all('*', async (req, res) => {
    await handler(req, res);
  });

  app.listen(port, '0.0.0.0', () => {
    console.log(`Serverless hello handler listening on http://localhost:${port}`);
  });
}
