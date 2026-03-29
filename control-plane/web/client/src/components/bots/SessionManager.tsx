import { useState, useEffect, useCallback, useRef } from 'react';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Session {
  id: string;
  name: string;
  sessionKey: string;
  createdAt: string;
}

interface SessionManagerProps {
  botId: string;
  activeSessionKey: string;
  onSessionSelect: (sessionKey: string) => void;
  className?: string;
}

interface UseSessionManagerReturn {
  sessions: Session[];
  activeSessionKey: string;
  selectSession: (key: string) => void;
  createSession: () => void;
  deleteSession: (key: string) => void;
  renameSession: (key: string, name: string) => void;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function storageKey(botId: string) {
  return `hanzo_bot_sessions_${botId}`;
}

function generateId(): string {
  return Date.now().toString(36);
}

function makeDefaultSession(botId: string): Session {
  return {
    id: 'main',
    name: 'Main',
    sessionKey: `${botId}:main`,
    createdAt: new Date().toISOString(),
  };
}

function loadSessions(botId: string): Session[] {
  try {
    const raw = localStorage.getItem(storageKey(botId));
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) return parsed;
    }
  } catch {
    // ignore corrupt data
  }
  return [makeDefaultSession(botId)];
}

function persistSessions(botId: string, sessions: Session[]) {
  localStorage.setItem(storageKey(botId), JSON.stringify(sessions));
}

// ---------------------------------------------------------------------------
// Hook — useSessionManager
// ---------------------------------------------------------------------------

export function useSessionManager(botId: string): UseSessionManagerReturn {
  const [sessions, setSessions] = useState<Session[]>(() => loadSessions(botId));
  const [activeSessionKey, setActiveSessionKey] = useState<string>(
    () => loadSessions(botId)[0]?.sessionKey ?? `${botId}:main`,
  );

  // Re-load when botId changes
  useEffect(() => {
    const loaded = loadSessions(botId);
    setSessions(loaded);
    setActiveSessionKey(loaded[0]?.sessionKey ?? `${botId}:main`);
  }, [botId]);

  // Persist whenever sessions change
  useEffect(() => {
    persistSessions(botId, sessions);
  }, [botId, sessions]);

  const selectSession = useCallback((key: string) => {
    setActiveSessionKey(key);
  }, []);

  const createSession = useCallback(() => {
    setSessions((prev) => {
      const n = prev.length + 1;
      const id = `session-${generateId()}`;
      const newSession: Session = {
        id,
        name: `Session ${n}`,
        sessionKey: `${botId}:${id}`,
        createdAt: new Date().toISOString(),
      };
      const next = [...prev, newSession];
      // auto-select the new session
      setActiveSessionKey(newSession.sessionKey);
      return next;
    });
  }, [botId]);

  const deleteSession = useCallback(
    (key: string) => {
      setSessions((prev) => {
        if (prev.length <= 1) return prev; // keep at least one
        const next = prev.filter((s) => s.sessionKey !== key);
        // if we deleted the active session, select the first remaining
        setActiveSessionKey((current) =>
          current === key ? next[0]?.sessionKey ?? `${botId}:main` : current,
        );
        return next;
      });
    },
    [botId],
  );

  const renameSession = useCallback((key: string, name: string) => {
    setSessions((prev) =>
      prev.map((s) => (s.sessionKey === key ? { ...s, name } : s)),
    );
  }, []);

  return {
    sessions,
    activeSessionKey,
    selectSession,
    createSession,
    deleteSession,
    renameSession,
  };
}

// ---------------------------------------------------------------------------
// Component — SessionManager
// ---------------------------------------------------------------------------

export function SessionManager({
  botId,
  activeSessionKey,
  onSessionSelect,
  className = '',
}: SessionManagerProps) {
  const {
    sessions,
    createSession,
    deleteSession,
    renameSession,
  } = useSessionManager(botId);

  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  // Focus input when editing starts
  useEffect(() => {
    if (editingKey && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editingKey]);

  const startEditing = (session: Session) => {
    setEditingKey(session.sessionKey);
    setEditValue(session.name);
  };

  const commitEdit = () => {
    if (editingKey && editValue.trim()) {
      renameSession(editingKey, editValue.trim());
    }
    setEditingKey(null);
  };

  const formatTime = (iso: string) => {
    try {
      const d = new Date(iso);
      return d.toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return '';
    }
  };

  return (
    <div
      className={`flex flex-col h-full w-[200px] bg-[#111118] border-r border-white/10 ${className}`}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-white/10">
        <span className="text-xs font-medium text-white/60 uppercase tracking-wider">
          Sessions
        </span>
        <button
          onClick={createSession}
          className="flex items-center justify-center w-5 h-5 rounded text-white/60 hover:text-white hover:bg-white/10 transition-colors"
          title="New session"
        >
          <svg
            width="12"
            height="12"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
          >
            <line x1="6" y1="1" x2="6" y2="11" />
            <line x1="1" y1="6" x2="11" y2="6" />
          </svg>
        </button>
      </div>

      {/* Session list */}
      <div className="flex-1 overflow-y-auto">
        {sessions.map((session) => {
          const isActive = session.sessionKey === activeSessionKey;
          const isEditing = editingKey === session.sessionKey;

          return (
            <div
              key={session.sessionKey}
              onClick={() => onSessionSelect(session.sessionKey)}
              onDoubleClick={() => startEditing(session)}
              className={`
                group relative px-3 py-2 cursor-pointer transition-colors
                border-l-2
                ${isActive
                  ? 'border-l-white bg-white/5'
                  : 'border-l-transparent hover:bg-white/[0.03]'
                }
              `}
            >
              {/* Session name */}
              {isEditing ? (
                <input
                  ref={inputRef}
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  onBlur={commitEdit}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') commitEdit();
                    if (e.key === 'Escape') setEditingKey(null);
                  }}
                  className="w-full bg-transparent text-sm text-white outline-none border-b border-white/30 pb-0.5"
                />
              ) : (
                <div className="text-sm text-white truncate pr-4">
                  {session.name}
                </div>
              )}

              {/* Timestamp */}
              <div className="text-[10px] text-white/40 mt-0.5">
                {formatTime(session.createdAt)}
              </div>

              {/* Delete button */}
              {sessions.length > 1 && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteSession(session.sessionKey);
                  }}
                  className="absolute right-2 top-2 hidden group-hover:flex items-center justify-center w-4 h-4 rounded text-white/30 hover:text-white/80 hover:bg-white/10 transition-colors"
                  title="Delete session"
                >
                  <svg
                    width="8"
                    height="8"
                    viewBox="0 0 8 8"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    strokeLinecap="round"
                  >
                    <line x1="1" y1="1" x2="7" y2="7" />
                    <line x1="7" y1="1" x2="1" y2="7" />
                  </svg>
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
