import dotenv from 'dotenv';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Bot, type AIConfig } from '@hanzo/playground';
import {
  aggregationRouter,
  decisionRouter,
  entityRouter,
  scenarioRouter,
  simulationRouter
} from './routers/index.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: resolve(process.cwd(), '.env') });
if (!process.env.OPENAI_API_KEY && !process.env.OPENROUTER_API_KEY) {
  dotenv.config({ path: resolve(__dirname, '../../../.env') });
}

const aiConfig: AIConfig =
  process.env.OPENROUTER_API_KEY
    ? {
        // OpenRouter is OpenAI-compatible; keep provider as openai
        provider: 'openai',
        model: process.env.OPENROUTER_MODEL ?? 'deepseek/deepseek-v3.1-terminus',
        apiKey: process.env.OPENROUTER_API_KEY,
        baseUrl: process.env.OPENROUTER_BASE_URL ?? 'https://openrouter.ai/api/v1'
      }
    : {
        provider: 'openai',
        model: process.env.OPENAI_MODEL ?? 'gpt-4o',
        apiKey: process.env.OPENAI_API_KEY
      };

const bot = new Bot({
  nodeId: 'simulation-engine',
  aiConfig,
  host: 'localhost',
  devMode: true
});

[scenarioRouter, entityRouter, decisionRouter, aggregationRouter, simulationRouter].forEach((router) =>
  bot.includeRouter(router)
);

bot.bot<{ message: string }, { echo: string }>('echo', async (ctx) => ({
  echo: ctx.input.message
}));

bot
  .serve()
  .then(() => {
    console.log('Simulation bot serving on port 8001');
  })
  .catch((err) => {
    console.error('Failed to start simulation bot', err);
    process.exit(1);
  });
