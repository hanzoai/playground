import { useSpaceStore } from '@/stores/spaceStore';

export function DeployLocalStep() {
  const activeSpace = useSpaceStore((s) => s.activeSpace);

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Run locally</h3>
        <p className="text-xs text-muted-foreground">
          Download the Hanzo Bot app or use the CLI to run a bot on your machine.
        </p>
      </div>

      <div className="space-y-3">
        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">Option A: Desktop App</p>
          <a
            href="https://hanzo.bot/download"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-3 py-1.5 bg-primary text-primary-foreground rounded-md text-xs font-medium hover:bg-primary/90"
          >
            Download Bot.app
          </a>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">Option B: CLI</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono">
{`hanzo bot run${activeSpace ? ` --space ${activeSpace.id}` : ''}`}
          </pre>
        </div>
      </div>
    </div>
  );
}
