import 'dotenv/config';
import { Agent } from '@playground/sdk';
import { reasonersRouter } from './reasoners.js';


async function main() {
  const agent = new Agent({
    nodeId: process.env.AGENT_ID ?? "init-example",
    playgroundUrl: process.env.AGENTS_URL ?? 'http://localhost:8080',
    port: Number(process.env.PORT ?? 8005),
    publicUrl: process.env.AGENT_CALLBACK_URL,
    version: '1.0.0',
    devMode: true,
    apiKey: process.env.AGENTS_API_KEY,
    aiConfig: {
      provider: 'openai',
      model: 'gpt-4o',
      apiKey: process.env.OPENAI_API_KEY,
    },
  });

  agent.includeRouter(reasonersRouter);

  await agent.serve();
  console.log(`Agent "${agent.config.nodeId}" listening on http://localhost:${agent.config.port}`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    // eslint-disable-next-line no-console
    console.error(err);
    process.exit(1);
  });
}
