import { useSpaceStore } from '@/stores/spaceStore';

export function DeployCLIStep() {
  const activeSpace = useSpaceStore((s) => s.activeSpace);
  const spaceId = activeSpace?.id ?? '<space-id>';

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Use the CLI</h3>
        <p className="text-xs text-muted-foreground">
          Run a headless bot agent from your terminal.
        </p>
      </div>

      <div className="space-y-3">
        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">1. Install the CLI</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono">
npm install -g @hanzo/cli
          </pre>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">2. Login</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono">
hanzo login
          </pre>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">3. Run a bot</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono">
{`hanzo bot run --space ${spaceId} --name my-bot`}
          </pre>
        </div>
      </div>
    </div>
  );
}
