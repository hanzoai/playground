/**
 * WalletConnect — wallet connection UI for AI coin earnings.
 *
 * Allows users to connect a crypto wallet to receive AI coin earned
 * by sharing unused AI capacity on the Hanzo network.
 */

import { useState } from 'react';
import { useNetworkStore } from '@/stores/networkStore';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { WalletProvider } from '@/types/network';

const PROVIDERS: { id: WalletProvider; label: string; icon: string }[] = [
  { id: 'hanzo',         label: 'Hanzo Wallet', icon: 'H' },
  { id: 'metamask',      label: 'MetaMask',     icon: 'M' },
  { id: 'walletconnect', label: 'WalletConnect', icon: 'W' },
  { id: 'coinbase',      label: 'Coinbase',     icon: 'C' },
];

export function WalletConnect() {
  const wallet = useNetworkStore((s) => s.wallet);
  const connect = useNetworkStore((s) => s.connectWallet);
  const disconnect = useNetworkStore((s) => s.disconnectWallet);

  const [address, setAddress] = useState('');
  const [selectedProvider, setSelectedProvider] = useState<WalletProvider>('hanzo');
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState('');

  async function handleConnect() {
    if (!address.trim()) {
      setError('Enter a wallet address');
      return;
    }
    if (!/^0x[a-fA-F0-9]{40}$/.test(address.trim())) {
      setError('Invalid Ethereum address (0x + 40 hex chars)');
      return;
    }
    setError('');
    setConnecting(true);
    try {
      await connect(selectedProvider, address.trim(), 1);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Connection failed');
    } finally {
      setConnecting(false);
    }
  }

  async function handleDisconnect() {
    await disconnect();
    setAddress('');
  }

  if (wallet) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Wallet Connected</CardTitle>
          <CardDescription>AI coin earnings are sent to this address.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-md border border-border/50 p-3">
            <div className="space-y-0.5">
              <p className="text-sm font-medium">{wallet.displayAddress}</p>
              <p className="text-xs text-muted-foreground capitalize">{wallet.provider}</p>
            </div>
            <Button variant="outline" size="sm" onClick={handleDisconnect}>
              Disconnect
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Connected {new Date(wallet.connectedAt).toLocaleDateString()}
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Connect Wallet</CardTitle>
        <CardDescription>
          Connect a wallet to receive AI coin earned from sharing unused capacity.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Provider buttons */}
        <div className="grid grid-cols-2 gap-2">
          {PROVIDERS.map((p) => (
            <Button
              key={p.id}
              variant={selectedProvider === p.id ? 'default' : 'outline'}
              size="sm"
              className="justify-start gap-2"
              onClick={() => setSelectedProvider(p.id)}
            >
              <span className="flex size-5 items-center justify-center rounded bg-muted text-[10px] font-bold">
                {p.icon}
              </span>
              {p.label}
            </Button>
          ))}
        </div>

        {/* Address input */}
        <div className="space-y-2">
          <Label htmlFor="wallet-address">Wallet Address</Label>
          <Input
            id="wallet-address"
            placeholder="0x..."
            value={address}
            onChange={(e) => { setAddress(e.target.value); setError(''); }}
            className="font-mono text-sm"
          />
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>

        <Button onClick={handleConnect} disabled={connecting} className="w-full">
          {connecting ? 'Connecting...' : 'Connect Wallet'}
        </Button>
      </CardContent>
    </Card>
  );
}
