import { useState, useEffect, useCallback } from 'react';
import { useSpaceStore } from '@/stores/spaceStore';
import {
  teamPlatformApi,
  teamPlatformStorage,
  type TeamPlatformConfig,
  type TeamWorkspace,
} from '@/services/teamPlatformApi';

export function TeamPlatformSection() {
  const activeSpaceId = useSpaceStore((s) => s.activeSpaceId);

  const [config, setConfig] = useState<TeamPlatformConfig | null>(null);
  const [accountUrl, setAccountUrl] = useState('');
  const [token, setToken] = useState('');
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState('');

  // Connected state
  const [workspaces, setWorkspaces] = useState<TeamWorkspace[]>([]);
  const [loadingWorkspaces, setLoadingWorkspaces] = useState(false);

  // Load persisted config on space change
  useEffect(() => {
    if (!activeSpaceId) {
      setConfig(null);
      return;
    }
    const saved = teamPlatformStorage.get(activeSpaceId);
    setConfig(saved);
    if (saved) {
      setAccountUrl(saved.accountUrl);
      setToken(saved.token);
    } else {
      setAccountUrl('');
      setToken('');
    }
  }, [activeSpaceId]);

  // Fetch workspaces when connected
  useEffect(() => {
    if (!config) {
      setWorkspaces([]);
      return;
    }
    setLoadingWorkspaces(true);
    teamPlatformApi
      .listWorkspaces(config.accountUrl, config.token)
      .then(setWorkspaces)
      .catch(() => setWorkspaces([]))
      .finally(() => setLoadingWorkspaces(false));
  }, [config]);

  const handleConnect = useCallback(async () => {
    if (!activeSpaceId || !accountUrl.trim() || !token.trim()) return;
    setConnecting(true);
    setError('');
    try {
      await teamPlatformApi.testConnection(accountUrl.trim(), token.trim());
      const newConfig: TeamPlatformConfig = { accountUrl: accountUrl.trim(), token: token.trim() };
      teamPlatformStorage.set(activeSpaceId, newConfig);
      setConfig(newConfig);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Connection failed');
    } finally {
      setConnecting(false);
    }
  }, [activeSpaceId, accountUrl, token]);

  const handleDisconnect = useCallback(() => {
    if (!activeSpaceId) return;
    teamPlatformStorage.remove(activeSpaceId);
    setConfig(null);
    setAccountUrl('');
    setToken('');
    setWorkspaces([]);
    setError('');
  }, [activeSpaceId]);

  const handleLinkWorkspace = useCallback(
    (workspaceId: string) => {
      if (!activeSpaceId || !config) return;
      const updated = { ...config, workspaceId: workspaceId || undefined };
      teamPlatformStorage.set(activeSpaceId, updated);
      setConfig(updated);
    },
    [activeSpaceId, config],
  );

  if (!activeSpaceId) return null;

  const isConnected = !!config;

  return (
    <section className="mb-8">
      <h2 className="text-lg font-semibold mb-3">Connected Platform</h2>

      {isConnected ? (
        <div className="border border-green-500/30 rounded-lg p-4 bg-green-500/5">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-green-500" />
              <span className="text-sm font-medium">Connected to Hanzo Team</span>
            </div>
            <button
              onClick={handleDisconnect}
              className="text-xs text-muted-foreground hover:text-destructive"
            >
              Disconnect
            </button>
          </div>

          <p className="text-xs text-muted-foreground mb-3 font-mono truncate">{config.accountUrl}</p>

          {/* Workspace selector */}
          <div>
            <label htmlFor="linked-workspace" className="block text-xs text-muted-foreground mb-1">
              Linked Workspace
            </label>
            {loadingWorkspaces ? (
              <p className="text-xs text-muted-foreground">Loading workspaces...</p>
            ) : (
              <select
                id="linked-workspace"
                value={config.workspaceId ?? ''}
                onChange={(e) => handleLinkWorkspace(e.target.value)}
                className="w-full px-3 py-2 border rounded-md bg-background text-sm"
              >
                <option value="">None</option>
                {workspaces.map((ws) => (
                  <option key={ws.id} value={ws.id}>
                    {ws.name}
                  </option>
                ))}
              </select>
            )}
          </div>
        </div>
      ) : (
        <div className="border rounded-lg p-4">
          <p className="text-sm text-muted-foreground mb-3">
            Connect this space to a Hanzo Team account for project context and collaboration.
          </p>
          <div className="space-y-2 mb-3">
            <div>
              <label htmlFor="team-account-url" className="block text-xs text-muted-foreground mb-1">
                Account URL
              </label>
              <input
                id="team-account-url"
                type="text"
                value={accountUrl}
                onChange={(e) => setAccountUrl(e.target.value)}
                placeholder="https://team.hanzo.ai/api/account"
                className="w-full px-3 py-2 border rounded-md bg-background text-sm"
              />
            </div>
            <div>
              <label htmlFor="team-token" className="block text-xs text-muted-foreground mb-1">
                Token
              </label>
              <input
                id="team-token"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="API token"
                className="w-full px-3 py-2 border rounded-md bg-background text-sm"
              />
            </div>
          </div>

          {error && <p className="text-sm text-destructive mb-3">{error}</p>}

          <button
            onClick={handleConnect}
            disabled={connecting || !accountUrl.trim() || !token.trim()}
            className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90 disabled:opacity-50"
          >
            {connecting ? 'Connecting...' : 'Connect'}
          </button>
        </div>
      )}
    </section>
  );
}
