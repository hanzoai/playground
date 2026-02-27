/**
 * NetworkPage — AI Capacity Marketplace.
 *
 * Main dashboard for the Hanzo network marketplace. Shows earnings,
 * network stats, sharing controls, and a "How It Works" FAQ.
 */

import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useNetworkStore } from '@/stores/networkStore';
import { NetworkStatusBadge } from '@/components/network/NetworkStatusBadge';
import { EarningsChart } from '@/components/network/EarningsChart';
import { WalletConnect } from '@/components/network/WalletConnect';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';

function MetricCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className="text-2xl font-bold tabular-nums mt-1">{value}</p>
        {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
      </CardContent>
    </Card>
  );
}

const FAQ_ITEMS = [
  {
    q: 'What is capacity sharing?',
    a: `When you connect AI provider API keys (OpenAI, Anthropic, etc.) to Hanzo Bot, you have access to
AI compute. Most users don't use their full capacity 24/7. Capacity sharing lets the Hanzo network
route requests through your idle API access — privately and securely — so others in the network
benefit from cheaper, distributed AI compute while you earn AI coin for contributing.`,
  },
  {
    q: 'How do I earn AI coin?',
    a: `Every request routed through your idle capacity earns you AI coin on the Hanzo mainnet. The amount
depends on the model used, request size, and current network demand. Your earnings accumulate
automatically and appear in your balance. Connect a wallet to withdraw to any EVM-compatible chain.`,
  },
  {
    q: 'When is my capacity shared?',
    a: `In Automatic mode (default), sharing activates after you've been idle for 1 hour — no bot executions,
no chat messages, no active sessions. The moment you start using your bots again, sharing pauses
instantly. You can also use Manual mode (toggle on/off yourself) or Scheduled mode (set specific days
and hours). Your own usage always takes priority.`,
  },
  {
    q: 'How much can I earn?',
    a: `Earnings vary based on network demand and the AI models your API keys support. On average,
contributors earn 0.15–0.30 AI coin per hour of sharing. Power users with GPU-enabled keys or
multiple provider accounts can earn significantly more. Check the earnings chart for your personal rate.`,
  },
  {
    q: 'Is my data private and secure?',
    a: `Absolutely. Hanzo routes requests through your API keys but never exposes your keys, credentials, or
personal data. All traffic is encrypted end-to-end. Requests are stateless — no conversation history,
user data, or context is stored on your node. Your API keys remain in your local Hanzo Bot installation
and are never transmitted to the network.`,
  },
  {
    q: 'How do I withdraw earnings?',
    a: `Connect an EVM-compatible wallet (MetaMask, WalletConnect, Coinbase Wallet, or Hanzo Wallet) in
Settings → Network. AI coin is distributed on the Hanzo mainnet and can be bridged to other chains.
Withdrawals process within 24 hours. There is no minimum withdrawal amount.`,
  },
  {
    q: 'How do I sell my Claude Code access?',
    a: `Go to Marketplace → Sell Capacity. Choose "Claude Code" as the capacity type, set your hourly rate
(e.g. $1.00/hr), and specify how many hours you want to offer. When a buyer purchases, the Hanzo
network routes their requests through your account via a secure proxy — your API keys stay local and
private. You earn USD revenue deposited directly to your balance.`,
  },
  {
    q: 'Can I list my custom-trained agent?',
    a: `Yes — go to Marketplace → Sell Capacity and choose "Custom Agent". Enter your agent's DID
(Decentralized Identifier), describe its capabilities and specialization, and set your hourly rate.
Buyers rent your agent and requests are routed through your infrastructure via a secure proxy.
Your agent's DID serves as a verifiable, tamper-proof identity on the Hanzo network.`,
  },
  {
    q: 'Can I resell purchased capacity?',
    a: `Yes — buy in bulk at lower rates and resell in smaller chunks at a markup. When you have an active
order with remaining capacity, click "Resell Remaining" to create a new listing. The resale is tracked
so buyers know the provenance. This creates arbitrage opportunities and helps distribute capacity
efficiently across the network.`,
  },
  {
    q: 'What is confidential computing?',
    a: `Confidential computing uses hardware-based Trusted Execution Environments (TEEs) to process data
in encrypted memory that even the host operator cannot read. On the Hanzo network, sellers running
NVIDIA Blackwell GPUs with I/O TEE or NVIDIA H100 with confidential mode provide fully private
inference — your prompts, data, and model outputs are never exposed, not even to the seller.
Listings with TEE attestation show a verified "Confidential Computing" badge.`,
  },
  {
    q: 'How does the proxy and S3 transfer work?',
    a: `When you purchase capacity, you receive a proxy endpoint (e.g. https://proxy.hanzo.ai/v1/ord-xxx).
Route your AI requests to this endpoint and they're forwarded through the seller's capacity privately.
For file-heavy workloads like ML training, an S3 transfer bucket is provisioned per order — upload
datasets and checkpoints, and the seller's VM processes them. All traffic is encrypted end-to-end.`,
  },
];

export function NetworkPage() {
  const navigate = useNavigate();
  const config = useNetworkStore((s) => s.sharingConfig);
  const earnings = useNetworkStore((s) => s.earnings);
  const earningsHistory = useNetworkStore((s) => s.earningsHistory);
  const aiCoinBalance = useNetworkStore((s) => s.aiCoinBalance);
  const aiCoinPending = useNetworkStore((s) => s.aiCoinPending);
  const networkStats = useNetworkStore((s) => s.networkStats);
  const wallet = useNetworkStore((s) => s.wallet);
  const marketplaceStats = useNetworkStore((s) => s.marketplaceStats);
  const setSharingEnabled = useNetworkStore((s) => s.setSharingEnabled);
  const syncFromBackend = useNetworkStore((s) => s.syncFromBackend);
  const refreshMarketplace = useNetworkStore((s) => s.refreshMarketplace);

  useEffect(() => {
    syncFromBackend().catch(() => {});
    refreshMarketplace().catch(() => {});
  }, [syncFromBackend, refreshMarketplace]);

  const stats = networkStats;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">AI Capacity Network</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Share unused AI capacity, earn AI coin on the Hanzo mainnet.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <NetworkStatusBadge />
          <Switch
            checked={config.enabled}
            onCheckedChange={setSharingEnabled}
            aria-label="Toggle capacity sharing"
          />
        </div>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <MetricCard
          label="AI Coin Balance"
          value={aiCoinBalance.toFixed(2)}
          sub={aiCoinPending > 0 ? `+${aiCoinPending.toFixed(2)} pending` : undefined}
        />
        <MetricCard
          label="Earning Rate"
          value={`${earnings.currentRatePerHour.toFixed(2)}/hr`}
          sub={config.enabled ? 'Currently earning' : 'Sharing disabled'}
        />
        <MetricCard
          label="Network Nodes"
          value={stats ? stats.activeNodes.toLocaleString() : '--'}
          sub={stats ? `${stats.totalContributors.toLocaleString()} total contributors` : undefined}
        />
        <MetricCard
          label="Your Rank"
          value={stats?.userRank ? `#${stats.userRank.toLocaleString()}` : '--'}
          sub={stats ? `of ${stats.totalContributors.toLocaleString()}` : undefined}
        />
      </div>

      {/* Wallet — required to receive rewards */}
      {!wallet && (
        <WalletConnect />
      )}

      {/* Marketplace quick-access */}
      <Card className="border-primary/30 bg-primary/5">
        <CardContent className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-sm font-medium">AI Capacity Marketplace</p>
            <p className="text-xs text-muted-foreground">
              Browse {marketplaceStats?.activeListings?.toLocaleString() ?? '—'} active listings — sell Claude Code, custom agents, API keys, and GPU capacity.
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <Button variant="outline" size="sm" onClick={() => navigate('/marketplace')}>
              Browse
            </Button>
            <Button size="sm" onClick={() => navigate('/marketplace/create')}>
              Sell Capacity
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Earnings chart */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Earnings — Last 30 Days</CardTitle>
        </CardHeader>
        <CardContent>
          <EarningsChart records={earningsHistory} />
        </CardContent>
      </Card>

      {/* Earnings history */}
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">Recent Earnings</CardTitle>
            <Button variant="link" size="sm" className="h-auto p-0" onClick={() => navigate('/settings/network')}>
              Settings
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {earningsHistory.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">
              No earnings yet. Enable capacity sharing to start earning.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="pb-2 font-medium">Date</th>
                    <th className="pb-2 font-medium">Amount</th>
                    <th className="pb-2 font-medium">Duration</th>
                    <th className="pb-2 font-medium">Type</th>
                  </tr>
                </thead>
                <tbody>
                  {earningsHistory.slice(0, 10).map((r) => (
                    <tr key={r.id} className="border-b border-border/30 last:border-0">
                      <td className="py-2 text-muted-foreground">
                        {new Date(r.timestamp).toLocaleDateString()}
                      </td>
                      <td className="py-2 font-medium tabular-nums">{r.amount.toFixed(2)} AI</td>
                      <td className="py-2 text-muted-foreground">
                        {(r.durationSeconds / 3600).toFixed(1)}h
                      </td>
                      <td className="py-2">
                        <Badge variant="secondary" className="text-[10px] capitalize">
                          {r.computeType}
                        </Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Network stats */}
      {stats && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Network Overview</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              <div>
                <p className="text-xs text-muted-foreground">Active Nodes</p>
                <p className="text-lg font-semibold tabular-nums">{stats.activeNodes.toLocaleString()}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">24h Distribution</p>
                <p className="text-lg font-semibold tabular-nums">{stats.totalDistributed24h.toLocaleString()} AI</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Network Throughput</p>
                <p className="text-lg font-semibold tabular-nums">{stats.networkTflops.toFixed(1)} TFLOPS</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Avg. Rate / Node / Hr</p>
                <p className="text-lg font-semibold tabular-nums">{stats.avgRatePerHour.toFixed(2)} AI</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Total Contributors</p>
                <p className="text-lg font-semibold tabular-nums">{stats.totalContributors.toLocaleString()}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Your Hours Shared</p>
                <p className="text-lg font-semibold tabular-nums">{earnings.totalHoursShared.toFixed(0)}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* FAQ */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">How It Works</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {FAQ_ITEMS.map((item, i) => (
            <Collapsible key={i}>
              <CollapsibleTrigger className="flex w-full items-center justify-between rounded-md px-3 py-2.5 text-sm font-medium hover:bg-muted/50 transition-colors text-left">
                {item.q}
                <span className="text-muted-foreground text-xs ml-2 shrink-0">+</span>
              </CollapsibleTrigger>
              <CollapsibleContent className="px-3 pb-3 text-sm text-muted-foreground leading-relaxed whitespace-pre-line">
                {item.a.trim()}
              </CollapsibleContent>
            </Collapsible>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
