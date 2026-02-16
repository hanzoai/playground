import { useState } from 'react';
import { useSpaceStore } from '@/stores/spaceStore';

export function DeployCloudStep() {
  const { activeSpace, registerNode } = useSpaceStore();
  const [deploying, setDeploying] = useState(false);
  const [done, setDone] = useState(false);

  const handleDeploy = async () => {
    if (!activeSpace) return;
    setDeploying(true);
    try {
      await registerNode({
        name: `cloud-${Date.now().toString(36)}`,
        type: 'cloud',
        os: 'linux',
      });
      setDone(true);
    } catch {
      // error handled by store
    } finally {
      setDeploying(false);
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Deploy to cloud</h3>
        <p className="text-xs text-muted-foreground">
          Provision a Node in your org's Kubernetes cluster.
        </p>
      </div>

      {done ? (
        <div className="border border-green-500/30 bg-green-500/5 rounded-lg p-3">
          <p className="text-xs text-green-600 dark:text-green-400 font-medium">
            Node provisioning started. It will appear in your space once online.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="border rounded-lg p-3 bg-muted/30">
            <p className="text-xs text-muted-foreground mb-3">
              This will deploy a <code className="text-xs font-mono bg-background px-1 rounded">hanzo/node</code> container
              into your org's DOKS cluster via Hanzo PaaS.
            </p>
            <button
              onClick={handleDeploy}
              disabled={deploying || !activeSpace}
              className="px-3 py-1.5 bg-primary text-primary-foreground rounded-md text-xs font-medium hover:bg-primary/90 disabled:opacity-50"
            >
              {deploying ? 'Provisioning...' : 'Deploy Node'}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
