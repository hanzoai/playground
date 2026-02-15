/**
 * ProvisionDialog
 *
 * Confirmation dialog before provisioning a team.
 * Shows the preset details and an optional workspace override.
 */

import { useState, useCallback } from 'react';
import type { TeamPreset } from '@/types/team';
import { cn } from '@/lib/utils';

interface ProvisionDialogProps {
  preset: TeamPreset | null;
  onConfirm: (presetId: string, workspace?: string) => void;
  onClose: () => void;
  loading?: boolean;
}

export function ProvisionDialog({ preset, onConfirm, onClose, loading }: ProvisionDialogProps) {
  const [workspace, setWorkspace] = useState('');

  const handleConfirm = useCallback(() => {
    if (!preset) return;
    onConfirm(preset.id, workspace || undefined);
  }, [preset, workspace, onConfirm]);

  if (!preset) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} />

      {/* Dialog */}
      <div className="relative w-full max-w-sm rounded-xl border border-border/60 bg-card shadow-2xl">
        {/* Header */}
        <div className="flex items-center gap-3 border-b border-border/40 px-5 py-4">
          <span className="text-2xl">{preset.emoji}</span>
          <div>
            <h2 className="text-sm font-semibold">Provision {preset.name}</h2>
            <p className="text-xs text-muted-foreground">{preset.bots.length} bots will be created</p>
          </div>
        </div>

        {/* Body */}
        <div className="px-5 py-4 space-y-3">
          {/* Bot list */}
          <div className="space-y-1.5">
            {preset.bots.map((bot) => (
              <div key={bot.role} className="flex items-center justify-between text-xs">
                <span className="font-medium">{bot.name}</span>
                <span className="text-muted-foreground font-mono text-[10px]">
                  {bot.model ?? 'default'}
                </span>
              </div>
            ))}
          </div>

          {/* Workspace */}
          <div>
            <label htmlFor="workspace" className="block text-xs text-muted-foreground mb-1">
              Workspace (optional)
            </label>
            <input
              id="workspace"
              type="text"
              value={workspace}
              onChange={(e) => setWorkspace(e.target.value)}
              placeholder="/workspace"
              className="w-full rounded-lg border border-border/50 bg-background px-3 py-2 text-xs placeholder:text-muted-foreground/60 outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/20"
            />
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 border-t border-border/40 px-5 py-3">
          <button
            type="button"
            onClick={onClose}
            disabled={loading}
            className="rounded-lg px-4 py-2 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors touch-manipulation"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={loading}
            className={cn(
              'rounded-lg px-4 py-2 text-xs font-medium transition-colors touch-manipulation',
              'bg-primary text-primary-foreground hover:bg-primary/90',
              loading && 'opacity-50 cursor-wait',
            )}
          >
            {loading ? 'Provisioning...' : 'Provision'}
          </button>
        </div>
      </div>
    </div>
  );
}
