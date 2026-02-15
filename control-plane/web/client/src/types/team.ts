/**
 * Team Types
 *
 * Teams are pre-configured groups of bots that can be provisioned
 * together. Each team preset defines a set of bot roles with
 * specific models and system prompts.
 */

export interface TeamPreset {
  id: string;
  name: string;
  description: string;
  emoji: string;
  bots: TeamPresetBot[];
  tags?: string[];
}

export interface TeamPresetBot {
  role: string;
  name: string;
  model?: string;
  systemPrompt?: string;
}

export interface Team {
  id: string;
  presetId: string;
  name: string;
  emoji: string;
  botIds: string[];
  provisionedAt: string;
  status: 'provisioning' | 'ready' | 'partial' | 'error';
}
