import 'dotenv/config';
import { Agent } from '../src/agent/Agent.js';
import { AgentRouter } from '../src/router/AgentRouter.js';
import { z } from 'zod';

console.log('Starting simulation example...');

const simulationRouter = new AgentRouter({ prefix: 'simulation' });

const SimulationResultSchema = z.object({
  scenario: z.string(),
  populationSize: z.number(),
  entities: z.array(z.any()),
  decisions: z.array(z.any()),
  insights: z.object({
    keyInsight: z.string(),
    outcomeDistribution: z.record(z.number())
  })
});

type SimulationResult = z.infer<typeof SimulationResultSchema>;

simulationRouter.reasoner<{
  scenario: string;
  populationSize: number;
  context?: string[];
  parallelBatchSize?: number;
  explorationRatio?: number;
}, SimulationResult>('runSimulation', async (ctx) => {
  const { scenario, populationSize, context = [], parallelBatchSize = 20 } = ctx.input;

  const scenarioAnalysis = await ctx.ai(`Analyze scenario: ${scenario}`);
  const factorGraph = await ctx.ai(`Build factor graph: ${scenarioAnalysis}`);

  await ctx.memory.set('last_scenario', { scenario, factorGraph });

  return {
    scenario,
    populationSize,
    entities: Array.from({ length: parallelBatchSize }).map((_, i) => ({ id: i })),
    decisions: [],
    insights: {
      keyInsight: 'Simulation complete',
      outcomeDistribution: { success: 0.8, failure: 0.2 }
    }
  };
});

simulationRouter.reasoner<{ scenario: string }, any>('decomposeScenario', async (ctx) => {
  return ctx.ai(`Decompose: ${ctx.input.scenario}`);
});

const agent = new Agent({
  nodeId: 'simulation-engine',
  aiConfig: { model: 'gpt-4o', provider: 'openai', apiKey: process.env.OPENAI_API_KEY },
  host: '127.0.0.1',
  devMode: true
});

agent.includeRouter(simulationRouter);

agent.reasoner<{ message: string }, { echo: string }>('echo', async (ctx) => ({
  echo: ctx.input.message
}));

agent
  .serve()
  .then(() => {
    console.log('Simulation agent serving on port 8001');
  })
  .catch((err) => {
    console.error('Failed to start simulation agent', err);
    process.exit(1);
  });
