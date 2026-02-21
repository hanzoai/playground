import { useEffect, useState } from 'react';
import { useSpaceStore } from '@/stores/spaceStore';
import { spaceApi } from '@/services/spaceApi';

export function SpacesPage() {
  const { spaces, activeSpaceId, loading, fetchSpaces, setActiveSpace, createSpace, deleteSpace } = useSpaceStore();
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDescription, setNewDescription] = useState('');
  const [memberCounts, setMemberCounts] = useState<Record<string, number>>({});

  useEffect(() => { fetchSpaces(); }, [fetchSpaces]);

  // Fetch member counts for all spaces to show "Shared" badges
  useEffect(() => {
    if (spaces.length === 0) return;
    Promise.allSettled(
      spaces.map(s => spaceApi.listMembers(s.id).then(r => ({ id: s.id, count: r.members.length })))
    ).then(results => {
      const counts: Record<string, number> = {};
      for (const r of results) {
        if (r.status === 'fulfilled') counts[r.value.id] = r.value.count;
      }
      setMemberCounts(counts);
    });
  }, [spaces]);

  const handleCreate = async () => {
    if (!newName.trim()) return;
    const space = await createSpace(newName, newDescription);
    await setActiveSpace(space.id);
    setNewName('');
    setNewDescription('');
    setShowCreate(false);
  };

  return (
    <div className="max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-heading-1">Spaces</h1>
          <p className="text-body text-muted-foreground">
            Project workspaces for organizing bots, nodes, and teams
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 text-sm font-medium"
        >
          New Space
        </button>
      </div>

      {showCreate && (
        <div className="border rounded-lg p-4 mb-6 bg-card">
          <h3 className="text-sm font-medium mb-3">Create Space</h3>
          <input
            type="text"
            placeholder="Space name"
            value={newName}
            onChange={e => setNewName(e.target.value)}
            className="w-full px-3 py-2 border rounded-md bg-background text-sm mb-2"
            autoFocus
          />
          <input
            type="text"
            placeholder="Description (optional)"
            value={newDescription}
            onChange={e => setNewDescription(e.target.value)}
            className="w-full px-3 py-2 border rounded-md bg-background text-sm mb-3"
          />
          <div className="flex gap-2">
            <button
              onClick={handleCreate}
              className="px-3 py-1.5 bg-primary text-primary-foreground rounded-md text-sm"
            >
              Create
            </button>
            <button
              onClick={() => setShowCreate(false)}
              className="px-3 py-1.5 border rounded-md text-sm"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {loading && <p className="text-sm text-muted-foreground">Loading...</p>}

      <div className="space-y-2">
        {spaces.map(space => (
          <div
            key={space.id}
            className={`border rounded-lg p-4 cursor-pointer transition-colors ${
              space.id === activeSpaceId
                ? 'border-primary bg-primary/5'
                : 'hover:border-primary/50'
            }`}
            onClick={() => setActiveSpace(space.id)}
          >
            <div className="flex items-center justify-between">
              <div>
                <h3 className="font-medium">{space.name}</h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {space.slug} &middot; {space.org_id}
                </p>
                {space.description && (
                  <p className="text-sm text-muted-foreground mt-1">{space.description}</p>
                )}
              </div>
              <div className="flex items-center gap-2">
                {(memberCounts[space.id] ?? 0) > 1 && (
                  <span className="text-xs bg-blue-500/10 text-blue-600 dark:text-blue-400 px-2 py-0.5 rounded">
                    Shared ({memberCounts[space.id]})
                  </span>
                )}
                {space.id === activeSpaceId && (
                  <span className="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded">
                    Active
                  </span>
                )}
                <button
                  onClick={e => { e.stopPropagation(); deleteSpace(space.id); }}
                  className="text-xs text-muted-foreground hover:text-destructive"
                >
                  Delete
                </button>
              </div>
            </div>
          </div>
        ))}

        {!loading && spaces.length === 0 && (
          <div className="text-center py-12 text-muted-foreground">
            <p className="text-lg mb-2">No spaces yet</p>
            <p className="text-sm">Create your first space to get started</p>
          </div>
        )}
      </div>
    </div>
  );
}
