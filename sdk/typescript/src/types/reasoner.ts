import type { BotContext } from '../context/BotContext.js';

export interface BotDefinition<TInput = any, TOutput = any> {
  name: string;
  handler: BotHandler<TInput, TOutput>;
  options?: BotOptions;
}

export type BotHandler<TInput = any, TOutput = any> = (
  ctx: BotContext<TInput>
) => Promise<TOutput> | TOutput;

export interface BotOptions {
  tags?: string[];
  description?: string;
  inputSchema?: any;
  outputSchema?: any;
  trackWorkflow?: boolean;
  memoryConfig?: any;
}
