/**
 * TeamLauncher
 *
 * Quick-launch modal for deploying a pre-configured AI team.
 * Shows the Hanzo team profiles with avatars, roles, and selection.
 * Creates bots on the canvas with profile personality as system prompt.
 */

import { useState, useCallback } from 'react';
import { TEAM_PROFILES, DEFAULT_TEAM_IDS, type AgentProfile } from '@/lib/agentProfiles';
import { useCanvasStore } from '@/stores/canvasStore';
import { cn } from '@/lib/utils';

interface TeamLauncherProps {
  open: boolean;
  onClose: () => void;
}

export function TeamLauncher({ open, onClose }: TeamLauncherProps) {
  const upsertBot = useCanvasStore((s) => s.upsertBot);
  const autoLayout = useCanvasStore((s) => s.autoLayout);
  const persist = useCanvasStore((s) => s.persist);

  const [selected, setSelected] = useState<Set<string>>(
    () => new Set(DEFAULT_TEAM_IDS)
  );

  const toggle = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const selectAll = useCallback(() => {
    setSelected(new Set(TEAM_PROFILES.map((p) => p.id)));
  }, []);

  const selectNone = useCallback(() => {
    setSelected(new Set());
  }, []);

  const handleLaunch = useCallback(() => {
    const profiles = TEAM_PROFILES.filter((p) => selected.has(p.id));
    for (const profile of profiles) {
      upsertBot(`team-${profile.id}`, {
        name: profile.name,
        emoji: profile.emoji,
        avatar: profile.avatar,
        role: profile.role,
        color: profile.color,
        personality: profile.personality,
        status: 'idle',
        source: 'cloud',
        skills: profile.skills,
      });
    }
    autoLayout();
    persist();
    onClose();
  }, [selected, upsertBot, autoLayout, persist, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} />

      {/* Dialog */}
      <div className="relative w-full max-w-lg rounded-xl border border-border/60 bg-card shadow-2xl max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border/40 px-5 py-4 shrink-0">
          <div>
            <h2 className="text-sm font-semibold">Launch AI Team</h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              Select team members to deploy on the canvas
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        {/* Select controls */}
        <div className="flex items-center gap-2 px-5 py-2 border-b border-border/20 shrink-0">
          <button
            type="button"
            onClick={selectAll}
            className="text-[10px] text-primary hover:underline"
          >
            Select all
          </button>
          <span className="text-muted-foreground/40 text-[10px]">|</span>
          <button
            type="button"
            onClick={selectNone}
            className="text-[10px] text-muted-foreground hover:text-foreground hover:underline"
          >
            None
          </button>
          <span className="ml-auto text-[10px] text-muted-foreground">
            {selected.size} selected
          </span>
        </div>

        {/* Profile list */}
        <div className="flex-1 min-h-0 overflow-y-auto px-5 py-3">
          <div className="grid grid-cols-1 gap-1.5">
            {TEAM_PROFILES.map((profile) => (
              <ProfileRow
                key={profile.id}
                profile={profile}
                selected={selected.has(profile.id)}
                onToggle={toggle}
              />
            ))}
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 border-t border-border/40 px-5 py-3 shrink-0">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg px-4 py-2 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors touch-manipulation"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleLaunch}
            disabled={selected.size === 0}
            className={cn(
              'rounded-lg px-4 py-2 text-xs font-medium transition-colors touch-manipulation',
              'bg-primary text-primary-foreground hover:bg-primary/90',
              selected.size === 0 && 'opacity-50 cursor-not-allowed',
            )}
          >
            Launch Team ({selected.size})
          </button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Profile Row
// ---------------------------------------------------------------------------

function ProfileRow({
  profile,
  selected,
  onToggle,
}: {
  profile: AgentProfile;
  selected: boolean;
  onToggle: (id: string) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onToggle(profile.id)}
      className={cn(
        'flex items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-all touch-manipulation',
        selected
          ? 'bg-primary/10 ring-1 ring-primary/30'
          : 'hover:bg-accent/50',
      )}
    >
      {/* Checkbox */}
      <div
        className={cn(
          'flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors',
          selected
            ? 'border-primary bg-primary text-primary-foreground'
            : 'border-border/60',
        )}
      >
        {selected && (
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
            <path d="M2 5l2.5 2.5L8 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        )}
      </div>

      {/* Emoji */}
      <span className="text-lg leading-none shrink-0">{profile.emoji}</span>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">{profile.name}</span>
          <span
            className="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
            style={{ color: profile.color, backgroundColor: `${profile.color}15` }}
          >
            {profile.role}
          </span>
        </div>
        <div className="text-xs text-muted-foreground truncate mt-0.5">
          {profile.personality}
        </div>
      </div>

      {/* Skills */}
      <div className="hidden sm:flex items-center gap-1 shrink-0">
        {profile.skills.slice(0, 2).map((skill) => (
          <span
            key={skill}
            className="text-[9px] px-1.5 py-0.5 rounded-md bg-muted/50 text-muted-foreground"
          >
            {skill}
          </span>
        ))}
      </div>
    </button>
  );
}
