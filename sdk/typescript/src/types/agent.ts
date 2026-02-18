/**
 * @deprecated Use types from './bot.js' instead.
 * This module is kept for backward compatibility only.
 */
export type {
  BotConfig as AgentConfig,
  NodeCapability as AgentCapability,
  BotState as AgentState,
  ServerHandler as AgentHandler,
  AIConfig,
  MemoryConfig,
  MemoryScope,
  MCPServerConfig,
  MCPConfig,
  BotCapability,
  SkillCapability,
  DiscoveryResponse,
  DiscoveryPagination,
  CompactCapability,
  CompactDiscoveryResponse,
  DiscoveryFormat,
  DiscoveryResult,
  DiscoveryOptions,
  HealthStatus,
  ServerlessEvent,
  ServerlessResponse,
  ServerlessAdapter,
  DeploymentType,
  Awaitable
} from './bot.js';
