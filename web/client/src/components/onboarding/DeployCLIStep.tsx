import { useGateway } from '@/hooks/useGateway';

export function DeployCLIStep() {
  const { gatewayUrl } = useGateway();
  const gwUrl = gatewayUrl ?? 'wss://gw.hanzo.bot';

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Use the CLI</h3>
        <p className="text-xs text-muted-foreground">
          One command to log in, connect, and start your node.
        </p>
      </div>

      <div className="space-y-3">
        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">Quick start</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono select-all">
npx @hanzo/bot
          </pre>
          <p className="text-[10px] text-muted-foreground mt-1.5">
            Handles login, configuration, and gateway connection automatically.
          </p>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">Install as system service</p>
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
      </p>
    </div>
  );
}
