import { describe, it, expect } from 'vitest';
import { Agent } from '../src/agent/Agent.js';
import { BotRouter } from '../src/router/BotRouter.js';
import type { MemoryChangeEvent } from '../src/memory/MemoryInterface.js';

describe('Agent', () => {
  it('registers bots and skills directly', () => {
    const agent = new Bot({ nodeId: 'test-agent', devMode: true });
    agent.bot('hello', async () => ({ ok: true }));
    agent.skill('format', () => ({ upper: 'X' }));

    expect(agent.bots.all().map((r) => r.name)).toContain('hello');
    expect(agent.skills.all().map((s) => s.name)).toContain('format');
  });

  it('includes routers with prefixes', () => {
    const router = new BotRouter({ prefix: 'simulation' });
    router.bot('run', async () => ({}));
    router.skill('format', () => ({}));

    const agent = new Bot({ nodeId: 'test-agent', devMode: true });
    agent.includeRouter(router);

    expect(agent.bots.all().map((r) => r.name)).toContain('simulation_run');
    expect(agent.skills.all().map((s) => s.name)).toContain('simulation_format');
  });

  it('calls local bot via agent.call when target matches node id', async () => {
    const agent = new Bot({ nodeId: 'local', devMode: true });
    agent.bot('echo', async (ctx) => ({ echo: ctx.input.msg }));

    const result = await agent.call('local.echo', { msg: 'hi' });
    expect(result).toEqual({ echo: 'hi' });
  });

  it('filters memory events by scope when dispatching watchers', () => {
    const agent = new Bot({ nodeId: 'watcher', devMode: true });
    const captured: MemoryChangeEvent[] = [];

    agent.watchMemory('order.*', (event) => captured.push(event), { scope: 'workflow' });
    agent.watchMemory('order.*', (event) => captured.push({ ...event, agentId: 'any' }));

    const event1: MemoryChangeEvent = {
      key: 'order.1',
      data: {},
      scope: 'workflow',
      scopeId: 'wf-1',
      timestamp: new Date().toISOString(),
      agentId: 'watcher'
    };
    const event2: MemoryChangeEvent = {
      ...event1,
      scope: 'session',
      scopeId: 's-1'
    };

    (agent as any).dispatchMemoryEvent(event1);
    (agent as any).dispatchMemoryEvent(event2);

    expect(captured.length).toBe(3);
    expect(captured[0].scope).toBe('workflow');
    expect(captured[1].scope).toBe('workflow');
    expect(captured[2].scope).toBe('session');
  });
});
