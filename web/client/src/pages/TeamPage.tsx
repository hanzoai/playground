/**
 * TeamPage
 *
 * Team management: browse presets, provision teams, view active teams.
 */

import { useEffect, useState, useCallback } from 'react';
import { TeamGrid } from '@/components/team/TeamGrid';
import { ProvisionDialog } from '@/components/team/ProvisionDialog';
import { useTeamStore } from '@/stores/teamStore';
import { useCanvasStore } from '@/stores/canvasStore';
import { teamPresetsList, teamProvision } from '@/services/gatewayApi';
import { useGateway } from '@/hooks/useGateway';
import type { TeamPreset } from '@/types/team';

export function TeamPage() {
  const { isConnected } = useGateway();
  const presets = useTeamStore((s) => s.presets);
  const teams = useTeamStore((s) => s.teams);
  const setPresets = useTeamStore((s) => s.setPresets);
  const addTeam = useTeamStore((s) => s.addTeam);
  const loading = useTeamStore((s) => s.loading);
  const setLoading = useTeamStore((s) => s.setLoading);
  const upsertBot = useCanvasStore((s) => s.upsertBot);

  const [selectedPreset, setSelectedPreset] = useState<TeamPreset | null>(null);
  const [provisioning, setProvisioning] = useState(false);

  // Fetch presets on connect
  useEffect(() => {
    if (!isConnected) return;
    setLoading(true);
    teamPresetsList()
      .then((result) => setPresets(result.presets ?? []))
      .catch(() => setPresets([]))
      .finally(() => setLoading(false));
  }, [isConnected, setPresets, setLoading]);

  const handleProvision = useCallback(async (presetId: string, workspace?: string) => {
    setProvisioning(true);
    try {
      const result = await teamProvision({ presetId, workspace });
      addTeam({
        id: result.teamId,
        presetId,
        name: selectedPreset?.name ?? 'Team',
        emoji: selectedPreset?.emoji ?? '🤝',
        botIds: result.agents.map((a) => a.id),
        provisionedAt: new Date().toISOString(),
        status: 'provisioning',
      });

      // Add bots to canvas
      for (const agent of result.agents) {
        upsertBot(agent.id, {
          name: agent.name,
          emoji: agent.emoji,
          avatar: agent.avatar,
          status: agent.status,
          sessionKey: agent.sessionKey,
          model: agent.model,
          workspace: agent.workspace,
          source: 'cloud',
          teamId: result.teamId,
        });
      }

      setSelectedPreset(null);
    } catch (e) {
      console.error('[TeamPage] Provision failed:', e);
    } finally {
      setProvisioning(false);
    }
  }, [addTeam, upsertBot, selectedPreset]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-xl font-semibold">Teams</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Provision pre-configured teams of bots for common workflows.
        </p>
      </div>

      {/* Connection warning */}
      {!isConnected && (
        <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/5 px-4 py-3 text-sm text-yellow-600 dark:text-yellow-400">
          Connect to the gateway to load team presets.
        </div>
      )}

      {/* Loading */}
      {loading && (
        <div className="flex items-center justify-center py-16 text-sm text-muted-foreground">
          Loading presets...
        </div>
      )}

      {/* Presets */}
      {!loading && (
        <div>
          <h2 className="text-sm font-medium mb-3">Presets</h2>
          <TeamGrid
            presets={presets}
            onProvision={setSelectedPreset}
            disabled={!isConnected}
          />
        </div>
      )}

      {/* Active teams */}
      {teams.length > 0 && (
        <div>
          <h2 className="text-sm font-medium mb-3">Active Teams</h2>
          <div className="space-y-2">
            {teams.map((team) => (
              <div
                key={team.id}
                className="flex items-center gap-3 rounded-lg border border-border/50 bg-card px-4 py-3"
              >
                <span className="text-xl">{team.emoji}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium">{team.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {team.botIds.length} bots — {team.status}
                  </div>
                </div>
                <span className="text-[10px] text-muted-foreground font-mono">
                  {new Date(team.provisionedAt).toLocaleTimeString()}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Provision dialog */}
      <ProvisionDialog
        preset={selectedPreset}
        onConfirm={handleProvision}
        onClose={() => setSelectedPreset(null)}
        loading={provisioning}
      />
    </div>
  );
}
