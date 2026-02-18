import type { BotRouter } from '../router/BotRouter.js';
import type { BotDefinition, BotHandler, BotOptions } from '../types/bot.js';

export class BotRegistry {
  private readonly bots = new Map<string, BotDefinition>();

  register<TInput = any, TOutput = any>(
    name: string,
    handler: BotHandler<TInput, TOutput>,
    options?: BotOptions
  ) {
    this.bots.set(name, { name, handler, options });
  }

  includeRouter(router: BotRouter) {
    router.bots.forEach((bot) => {
      this.bots.set(bot.name, bot);
    });
  }

  get(name: string) {
    return this.bots.get(name);
  }

  all() {
    return Array.from(this.bots.values());
  }
}
