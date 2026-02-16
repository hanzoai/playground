import type { BotDefinition, BotHandler, BotOptions } from '../types/bot.js';
import type { SkillDefinition, SkillHandler, SkillOptions } from '../types/skill.js';

export interface BotRouterOptions {
  prefix?: string;
  tags?: string[];
}

export class BotRouter {
  readonly prefix?: string;
  readonly tags?: string[];
  readonly bots: BotDefinition[] = [];
  readonly skills: SkillDefinition[] = [];

  constructor(options: BotRouterOptions = {}) {
    this.prefix = options.prefix;
    this.tags = options.tags;
  }

  bot<TInput = any, TOutput = any>(
    name: string,
    handler: BotHandler<TInput, TOutput>,
    options?: BotOptions
  ) {
    const fullName = this.prefix ? `${sanitize(this.prefix)}_${name}` : name;
    this.bots.push({ name: fullName, handler, options });
    return this;
  }

  skill<TInput = any, TOutput = any>(
    name: string,
    handler: SkillHandler<TInput, TOutput>,
    options?: SkillOptions
  ) {
    const fullName = this.prefix ? `${sanitize(this.prefix)}_${name}` : name;
    this.skills.push({ name: fullName, handler, options });
    return this;
  }
}

function sanitize(value: string) {
  return value.replace(/[^0-9a-zA-Z]+/g, '_').replace(/_+/g, '_').replace(/^_+|_+$/g, '');
}
