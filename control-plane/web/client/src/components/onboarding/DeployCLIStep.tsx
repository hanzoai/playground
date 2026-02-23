import { useSpaceStore } from '@/stores/spaceStore';
import { useGateway } from '@/hooks/useGateway';

export function DeployCLIStep() {
  const activeSpace = useSpaceStore((s) => s.activeSpace);
  const { gatewayUrl } = useGateway();
  const gwUrl = gatewayUrl ?? 'wss://gw.hanzo.bot';

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Use the CLI</h3>
        <p className="text-xs text-muted-foreground">
          Run a headless bot node from your terminal.
        </p>
      </div>

      <div className="space-y-3">
        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">1. Install</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono select-all">
npm install -g @hanzo/bot
          </pre>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">2. Run as foreground process</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono select-all">
{`hanzo-bot node run`}
          </pre>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">Or install as system service</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono select-all">
hanzo-bot node install
          </pre>
          <p className="text-[10px] text-muted-foreground mt-1.5">
            Uses launchd (macOS) / systemd (Linux) to run at startup.
          </p>
        </div>
      </div>

      <p className="text-[10px] text-muted-foreground">
        Gateway: <code className="text-[10px]">{gwUrl}</code>
        {activeSpace ? ` | Space: ${activeSpace.id}` : ''}
      </p>
    </div>
  );
}
