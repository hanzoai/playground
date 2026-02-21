/**
 * GatewaySettings Component
 *
 * Allows users to configure a custom gateway URL and token,
 * enabling the cloud playground (app.hanzo.bot) to connect to
 * a user's local bot gateway via a tunnel.
 */

import { useState, useCallback } from 'react';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import {
  Wifi,
  WifiOff,
  Eye,
  EyeOff,
  Reset,
  Link,
} from '@/components/ui/icon-bridge';
import { useSettingsStore } from '@/stores/settingsStore';
import { useGateway } from '@/hooks/useGateway';
import { gateway } from '@/services/gatewayClient';

const DEFAULT_GATEWAY_URL = 'wss://bot.hanzo.ai';

function isValidGatewayUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return ['ws:', 'wss:'].includes(parsed.protocol);
  } catch {
    return false;
  }
}

export function GatewaySettings() {
  const { connectionState, gatewayUrl: activeUrl } = useGateway();
  const { gatewayUrl, gatewayToken, setGatewayUrl, setGatewayToken, reset } = useSettingsStore();

  const [urlDraft, setUrlDraft] = useState(gatewayUrl ?? '');
  const [tokenDraft, setTokenDraft] = useState(gatewayToken ?? '');
  const [showToken, setShowToken] = useState(false);
  const [urlError, setUrlError] = useState<string | null>(null);

  const isCustom = !!gatewayUrl;
  const isConnected = connectionState === 'connected';
  const isConnecting = connectionState === 'connecting' || connectionState === 'reconnecting' || connectionState === 'authenticating';

  const handleConnect = useCallback(() => {
    const trimmedUrl = urlDraft.trim();
    if (!trimmedUrl) {
      setUrlError('Gateway URL is required');
      return;
    }
    if (!isValidGatewayUrl(trimmedUrl)) {
      setUrlError('URL must use ws:// or wss:// protocol');
      return;
    }
    setUrlError(null);
    setGatewayUrl(trimmedUrl);
    setGatewayToken(tokenDraft.trim() || null);
    // The useGateway hook will auto-reconnect when the store changes
  }, [urlDraft, tokenDraft, setGatewayUrl, setGatewayToken]);

  const handleDisconnect = useCallback(() => {
    gateway.disconnect();
  }, []);

  const handleReset = useCallback(() => {
    reset();
    setUrlDraft('');
    setTokenDraft('');
    setUrlError(null);
    // The useGateway hook will auto-reconnect to the default URL
  }, [reset]);

  const statusBadge = (() => {
    switch (connectionState) {
      case 'connected':
        return <Badge variant="success">Connected</Badge>;
      case 'connecting':
      case 'authenticating':
        return <Badge variant="running">Connecting</Badge>;
      case 'reconnecting':
        return <Badge variant="pending">Reconnecting</Badge>;
      case 'error':
        return <Badge variant="failed">Error</Badge>;
      case 'disconnected':
      default:
        return <Badge variant="unknown">Disconnected</Badge>;
    }
  })();

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-heading-1 mb-1">Settings</h2>
        <p className="text-body">System configuration and preferences</p>
      </div>

      <Card interactive={false}>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Link size={20} weight="bold" />
              <CardTitle>Gateway Connection</CardTitle>
            </div>
            {statusBadge}
          </div>
          <CardDescription>
            Configure a custom gateway URL to connect to your local bot via a tunnel.
            Leave empty to use the default gateway ({DEFAULT_GATEWAY_URL}).
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Active connection info */}
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            {isConnected ? (
              <Wifi size={16} className="text-emerald-500" />
            ) : (
              <WifiOff size={16} className="text-muted-foreground" />
            )}
            <span>
              Active: <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">{activeUrl}</code>
            </span>
            {isCustom && (
              <Badge variant="metadata">custom</Badge>
            )}
          </div>

          {/* Gateway URL */}
          <div className="space-y-2">
            <Label htmlFor="gateway-url">Gateway URL</Label>
            <Input
              id="gateway-url"
              type="url"
              placeholder={DEFAULT_GATEWAY_URL}
              value={urlDraft}
              onChange={(e) => {
                setUrlDraft(e.target.value);
                setUrlError(null);
              }}
            />
            {urlError && (
              <p className="text-xs text-destructive">{urlError}</p>
            )}
            <p className="text-xs text-muted-foreground">
              The WebSocket URL of your bot gateway (e.g. wss://xxx.trycloudflare.com)
            </p>
          </div>

          {/* Gateway Token */}
          <div className="space-y-2">
            <Label htmlFor="gateway-token">Gateway Token</Label>
            <div className="relative">
              <Input
                id="gateway-token"
                type={showToken ? 'text' : 'password'}
                placeholder="Optional authentication token"
                value={tokenDraft}
                onChange={(e) => setTokenDraft(e.target.value)}
                className="pr-10"
              />
              <button
                type="button"
                onClick={() => setShowToken(!showToken)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                {showToken ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
            <p className="text-xs text-muted-foreground">
              The auth token configured on your bot gateway (gateway.auth.token)
            </p>
          </div>
        </CardContent>

        <CardFooter className="flex justify-between gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleReset}
            disabled={!isCustom}
          >
            <Reset size={14} />
            Reset to Default
          </Button>

          <div className="flex gap-2">
            {isConnected && (
              <Button
                variant="outline"
                size="sm"
                onClick={handleDisconnect}
              >
                Disconnect
              </Button>
            )}
            <Button
              size="sm"
              onClick={handleConnect}
              disabled={isConnecting}
            >
              {isConnecting ? 'Connecting...' : 'Connect'}
            </Button>
          </div>
        </CardFooter>
      </Card>
    </div>
  );
}
