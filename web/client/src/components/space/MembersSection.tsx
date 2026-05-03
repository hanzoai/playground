import { useState } from 'react';
import { useSpaceStore } from '@/stores/spaceStore';

const ROLES = ['owner', 'admin', 'member', 'viewer'] as const;

export function MembersSection() {
  const { members, addMember, removeMember } = useSpaceStore();
  const [userId, setUserId] = useState('');
  const [role, setRole] = useState<string>('member');
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState('');

  const handleAdd = async () => {
    if (!userId.trim()) return;
    setAdding(true);
    setError('');
    try {
      await addMember(userId.trim(), role);
      setUserId('');
      setRole('member');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to add member');
    } finally {
      setAdding(false);
    }
  };

  return (
    <section className="mb-8">
      <h2 className="text-lg font-semibold mb-3">Members ({members.length})</h2>

      {/* Add member form */}
      <div className="flex items-end gap-2 mb-4">
        <div className="flex-1">
          <label htmlFor="member-user-id" className="block text-xs text-muted-foreground mb-1">
            User ID
          </label>
          <input
            id="member-user-id"
            type="text"
            value={userId}
            onChange={e => setUserId(e.target.value)}
            placeholder="user@example.com or user ID"
            className="w-full px-3 py-2 border rounded-md bg-background text-sm"
          />
        </div>
        <div>
          <label htmlFor="member-role" className="block text-xs text-muted-foreground mb-1">
            Role
          </label>
          <select
            id="member-role"
            value={role}
            onChange={e => setRole(e.target.value)}
            className="px-3 py-2 border rounded-md bg-background text-sm"
          >
            {ROLES.map(r => (
              <option key={r} value={r}>{r}</option>
            ))}
          </select>
        </div>
        <button
          onClick={handleAdd}
          disabled={adding || !userId.trim()}
          className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90 disabled:opacity-50"
        >
          {adding ? 'Adding...' : 'Add'}
        </button>
      </div>

      {error && (
        <p className="text-sm text-destructive mb-3">{error}</p>
      )}

      {/* Member list */}
      {members.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No members yet. Add collaborators to share this space.
        </p>
      ) : (
        <div className="space-y-2">
          {members.map(member => (
            <div key={member.user_id} className="border rounded-lg p-3 flex items-center justify-between">
              <div>
                <span className="font-medium text-sm">{member.user_id}</span>
                <span className="text-xs text-muted-foreground ml-2 capitalize">{member.role}</span>
              </div>
              <button
                onClick={() => removeMember(member.user_id)}
                className="text-xs text-muted-foreground hover:text-destructive"
              >
                Remove
              </button>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
