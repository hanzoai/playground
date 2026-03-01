import { useEffect } from 'react';
import { useSpaceStore } from '@/stores/spaceStore';
import { MembersSection } from '@/components/space/MembersSection';
import { TeamPlatformSection } from '@/components/space/TeamPlatformSection';

export function SpaceSettingsPage() {
  const { activeSpace, nodes, bots, fetchSpaces, fetchNodes, fetchBots, fetchMembers } = useSpaceStore();

  // Bootstrap active space on direct navigation (e.g. refresh on /spaces/settings)
  useEffect(() => {
    if (!activeSpace) fetchSpaces();
  }, [activeSpace, fetchSpaces]);

  useEffect(() => {
    fetchNodes();
    fetchBots();
    fetchMembers();
  }, [fetchNodes, fetchBots, fetchMembers]);

  if (!activeSpace) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-muted-foreground">Loading space...</p>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-0">
      <h1 className="text-heading-1 mb-1">{activeSpace.name}</h1>
      <p className="text-sm text-muted-foreground mb-6">
        {activeSpace.slug} &middot; {activeSpace.org_id}
      </p>

      {/* Nodes */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-3">Nodes ({nodes?.length ?? 0})</h2>
        {(nodes?.length ?? 0) === 0 ? (
          <p className="text-sm text-muted-foreground">
            No nodes registered. Connect a local node or deploy to cloud.
          </p>
        ) : (
          <div className="space-y-2">
            {(nodes ?? []).map(node => (
              <div key={node.node_id} className="border rounded-lg p-3 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <span className="font-medium text-sm truncate block">{node.name || node.node_id}</span>
                  <span className="text-xs text-muted-foreground">
                    {node.type} &middot; {node.os || 'unknown'} &middot; {node.status}
                  </span>
                </div>
                <span className={`w-2 h-2 rounded-full ${
                  node.status === 'online' ? 'bg-green-500' :
                  node.status === 'provisioning' ? 'bg-yellow-500' : 'bg-gray-400'
                }`} />
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Bots */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-3">Bots ({bots?.length ?? 0})</h2>
        {(bots?.length ?? 0) === 0 ? (
          <p className="text-sm text-muted-foreground">
            No bots deployed. Go to the canvas to deploy your first bot.
          </p>
        ) : (
          <div className="space-y-2">
            {(bots ?? []).map(bot => (
              <div key={bot.bot_id} className="border rounded-lg p-3 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <span className="font-medium text-sm truncate block">{bot.name}</span>
                  <span className="text-xs text-muted-foreground">
                    {bot.model || 'default'} &middot; {bot.view} &middot; {bot.status}
                  </span>
                </div>
                <span className="text-xs text-muted-foreground shrink-0 truncate">
                  on {bot.node_id}
                </span>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Members */}
      <MembersSection />

      {/* Connected Platform */}
      <TeamPlatformSection />

      {/* Space Info */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-3">Details</h2>
        <dl className="text-sm space-y-1">
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24">ID:</dt>
            <dd className="font-mono text-xs">{activeSpace.id}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24">Org:</dt>
            <dd>{activeSpace.org_id}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24">Created by:</dt>
            <dd>{activeSpace.created_by}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-muted-foreground w-24">Created:</dt>
            <dd>{new Date(activeSpace.created_at).toLocaleDateString()}</dd>
          </div>
        </dl>
      </section>
    </div>
  );
}
