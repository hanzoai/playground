import { useSpaceStore } from '@/stores/spaceStore';
import { useGateway } from '@/hooks/useGateway';

export function DeployLocalStep() {
  const activeSpace = useSpaceStore((s) => s.activeSpace);
  const { gatewayUrl } = useGateway();
  const gwUrl = gatewayUrl ?? 'wss://gw.hanzo.bot';

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Connect your machine</h3>
        <p className="text-xs text-muted-foreground">
          Run a bot node on your computer that connects to the gateway.
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
          <p className="text-xs font-medium mb-2">2. Configure</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono select-all">
hanzo-bot configure
          </pre>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">3. Connect</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono select-all">
{`hanzo-bot node run`}
          </pre>
          <p className="text-[10px] text-muted-foreground mt-1.5">
            Connects to <code className="text-[10px]">{gwUrl}</code>
            {activeSpace ? ` in space ${activeSpace.id}` : ''}
          </p>
        </div>
      </div>
    </div>
  );
}
