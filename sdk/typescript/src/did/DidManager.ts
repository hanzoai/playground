import {
  DidClient,
  type DIDIdentityPackage,
  type DIDRegistrationRequest
} from './DidClient.js';

/**
 * Manages DID registration and identity package storage for an agent.
 *
 * This class handles:
 * - Registering the agent with the DID system
 * - Storing the identity package (agent DID, bot DIDs, skill DIDs)
 * - Resolving DIDs for specific functions (bots/skills)
 */
export class DidManager {
  private readonly client: DidClient;
  private readonly agentNodeId: string;
  private identityPackage?: DIDIdentityPackage;
  private _enabled = false;

  constructor(client: DidClient, agentNodeId: string) {
    this.client = client;
    this.agentNodeId = agentNodeId;
  }

  /**
   * Register agent with the DID system and obtain identity package.
   *
   * @param bots - List of bot definitions
   * @param skills - List of skill definitions
   * @returns true if registration succeeded
   */
  async registerAgent(
    bots: Array<{ id: string; [key: string]: any }>,
    skills: Array<{ id: string; [key: string]: any }>
  ): Promise<boolean> {
    const request: DIDRegistrationRequest = {
      agentNodeId: this.agentNodeId,
      bots,
      skills
    };

    const response = await this.client.registerAgent(request);

    if (response.success && response.identityPackage) {
      this.identityPackage = response.identityPackage;
      this._enabled = true;
      return true;
    }

    console.warn(`[DID] Registration failed: ${response.error ?? 'Unknown error'}`);
    return false;
  }

  /**
   * Check if DID system is enabled and identity package is available.
   */
  get enabled(): boolean {
    return this._enabled && this.identityPackage !== undefined;
  }

  /**
   * Get the agent node DID.
   */
  getAgentDid(): string | undefined {
    return this.identityPackage?.agentDid.did;
  }

  /**
   * Get DID for a specific function (bot or skill).
   * Falls back to agent DID if function not found.
   *
   * @param functionName - Name of the bot or skill
   * @returns DID string or undefined if not registered
   */
  getFunctionDid(functionName: string): string | undefined {
    if (!this.identityPackage) {
      return undefined;
    }

    // Check bots first
    const botDid = this.identityPackage.botDids[functionName];
    if (botDid) {
      return botDid.did;
    }

    // Check skills
    const skillDid = this.identityPackage.skillDids[functionName];
    if (skillDid) {
      return skillDid.did;
    }

    // Fall back to agent DID
    return this.identityPackage.agentDid.did;
  }

  /**
   * Get the full identity package (for debugging/inspection).
   */
  getIdentityPackage(): DIDIdentityPackage | undefined {
    return this.identityPackage;
  }

  /**
   * Get a summary of the identity for debugging/monitoring.
   */
  getIdentitySummary(): Record<string, any> {
    if (!this.identityPackage) {
      return { enabled: false, message: 'No identity package available' };
    }

    return {
      enabled: true,
      agentDid: this.identityPackage.agentDid.did,
      playgroundServerId: this.identityPackage.playgroundServerId,
      botCount: Object.keys(this.identityPackage.botDids).length,
      skillCount: Object.keys(this.identityPackage.skillDids).length,
      botDids: Object.fromEntries(
        Object.entries(this.identityPackage.botDids).map(([name, identity]) => [name, identity.did])
      ),
      skillDids: Object.fromEntries(
        Object.entries(this.identityPackage.skillDids).map(([name, identity]) => [name, identity.did])
      )
    };
  }
}
