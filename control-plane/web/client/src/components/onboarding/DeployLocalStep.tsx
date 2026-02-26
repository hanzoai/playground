import { useGateway } from '@/hooks/useGateway';

export function DeployLocalStep() {
  const { gatewayUrl } = useGateway();
  const gwUrl = gatewayUrl ?? 'wss://gw.hanzo.bot';

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">Connect your machine</h3>
        <p className="text-xs text-muted-foreground">
          Run a single command to log in, connect your machine to the cloud, and start using the playground.
        </p>
      </div>

      <div className="space-y-3">
        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">Run this in your terminal</p>
          <pre className="text-xs bg-background border rounded p-2 overflow-x-auto font-mono select-all">
npx @hanzo/bot
          </pre>
          <p className="text-[10px] text-muted-foreground mt-1.5">
            Logs you in via hanzo.id, saves config, and connects to{' '}
            <code className="text-[10px]">{gwUrl}</code>
          </p>
        </div>

        <div className="border rounded-lg p-3 bg-muted/30">
          <p className="text-xs font-medium mb-2">Download the app</p>
          <div className="flex gap-2">
            <a
              href="https://github.com/hanzoai/bot/releases/latest"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-primary hover:underline"
            >
              macOS
            </a>
            <span className="text-xs text-muted-foreground">|</span>
            <a
              href="https://github.com/hanzoai/bot/releases/latest"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-primary hover:underline"
            >
              Windows
            </a>
            <span className="text-xs text-muted-foreground">|</span>
            <a
              href="https://github.com/hanzoai/bot/releases/latest"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-primary hover:underline"
            >
              Linux
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
