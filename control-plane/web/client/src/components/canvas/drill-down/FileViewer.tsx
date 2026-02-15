/**
 * FileViewer
 *
 * Shows files from the bot's workspace via gateway RPC.
 */

import { useState, useEffect, useCallback } from 'react';
import { gateway } from '@/services/gatewayClient';
import { cn } from '@/lib/utils';

interface FileEntry {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size?: number;
  modified?: string;
}

interface FileViewerProps {
  agentId: string;
  className?: string;
}

export function FileViewer({ agentId, className }: FileViewerProps) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Load file list
  useEffect(() => {
    setLoading(true);
    gateway.rpc<{ files: FileEntry[] }>('agents.files.list', { agentId })
      .then((result) => setFiles(result.files ?? []))
      .catch(() => setFiles([]))
      .finally(() => setLoading(false));
  }, [agentId]);

  // Load file content
  const loadFile = useCallback(async (path: string) => {
    setSelected(path);
    setContent(null);
    try {
      const result = await gateway.rpc<{ content: string }>('agents.files.get', { agentId, path });
      setContent(result.content);
    } catch {
      setContent('// Failed to load file');
    }
  }, [agentId]);

  return (
    <div className={cn('flex h-full', className)}>
      {/* File tree */}
      <div className="w-1/3 min-w-[120px] border-r border-border/30 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
            Loading...
          </div>
        ) : files.length === 0 ? (
          <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
            No files
          </div>
        ) : (
          <div className="py-1">
            {files.map((file) => (
              <button
                key={file.path}
                type="button"
                onClick={() => file.type === 'file' && loadFile(file.path)}
                className={cn(
                  'flex items-center gap-1.5 w-full px-2 py-1 text-[11px] text-left transition-colors',
                  selected === file.path ? 'bg-primary/10 text-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-accent/30',
                  file.type === 'directory' && 'font-medium'
                )}
              >
                <span className="text-[10px]">{file.type === 'directory' ? '📁' : '📄'}</span>
                <span className="truncate">{file.name}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* File content */}
      <div className="flex-1 overflow-auto">
        {content !== null ? (
          <pre className="p-3 text-[11px] font-mono text-foreground/90 leading-relaxed whitespace-pre-wrap">
            {content}
          </pre>
        ) : (
          <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
            Select a file
          </div>
        )}
      </div>
    </div>
  );
}
