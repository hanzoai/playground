import 'dotenv/config';
import { Bot } from '@hanzo/playground';
import { botsRouter } from './bots.js';


async function main() {
  const bot = new Bot({
    nodeId: process.env.HANZO_NODE_ID ?? "init-example",
    playgroundUrl: process.env.PLAYGROUND_URL ?? 'http://localhost:8080',
    port: Number(process.env.PORT ?? 8005),
    publicUrl: process.env.HANZO_CALLBACK_URL,
    version: '1.0.0',
    devMode: true,
    apiKey: process.env.PLAYGROUND_API_KEY,
    aiConfig: {
      provider: 'openai',
      model: 'gpt-4o',
      apiKey: process.env.OPENAI_API_KEY,
    },
  });

  bot.includeRouter(botsRouter);

  await bot.serve();
  console.log(`Bot "${bot.config.nodeId}" listening on http://localhost:${bot.config.port}`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    // eslint-disable-next-line no-console
    console.error(err);
    process.exit(1);
  });
}
