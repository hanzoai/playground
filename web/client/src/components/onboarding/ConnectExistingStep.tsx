import { useState } from 'react';
import { useSpaceStore } from '@/stores/spaceStore';

export function ConnectExistingStep() {
  const { activeSpace, registerNode } = useSpaceStore();
  const [endpoint, setEndpoint] = useState('');
  const [name, setName] = useState('');
  const [connecting, setConnecting] = useState(false);
  const [done, setDone] = useState(false);

  const handleConnect = async () => {
    if (!activeSpace || !endpoint.trim()) return;
    setConnecting(true);
    try {
      await registerNode({
        name: name.trim() || 'custom-node',
        type: 'local',
        endpoint: endpoint.trim(),
      });
      setDone(true);
    } catch {
      // error handled by store
    } finally {
      setConnecting(false);
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Connect existing node</h3>
        <p className="text-xs text-muted-foreground">
          Register a running Node by entering its API endpoint.
        </p>
      </div>

      {done ? (
        <div className="border border-green-500/30 bg-green-500/5 rounded-lg p-3">
          <p className="text-xs text-green-600 dark:text-green-400 font-medium">
            Node registered. It will appear once the heartbeat is confirmed.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          <input
            type="text"
            placeholder="Node name (optional)"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border rounded-md bg-background text-sm"
          />
          <input
            type="url"
            placeholder="https://node.example.com:9550"
            value={endpoint}
            onChange={(e) => setEndpoint(e.target.value)}
            className="w-full px-3 py-2 border rounded-md bg-background text-sm"
          />
          <button
            onClick={handleConnect}
            disabled={connecting || !endpoint.trim() || !activeSpace}
            className="px-3 py-1.5 bg-primary text-primary-foreground rounded-md text-xs font-medium hover:bg-primary/90 disabled:opacity-50"
          >
            {connecting ? 'Connecting...' : 'Connect Node'}
          </button>
        </div>
      )}
    </div>
  );
}
