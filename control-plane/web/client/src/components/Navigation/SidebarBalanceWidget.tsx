/**
 * SidebarBalanceWidget — dual AI coin + USD balance in sidebar footer.
 *
 * Shows both balances in the sidebar footer above the user menu.
 * In collapsed state shows just an icon with a tooltip.
 * Click navigates to the Network marketplace page.
 */

import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useSidebar } from '@/components/ui/sidebar';
import { SidebarMenu, SidebarMenuItem, SidebarMenuButton } from '@/components/ui/sidebar';
import { useNetworkStore } from '@/stores/networkStore';
import { getBalance } from '@/services/billingApi';
import { getBotWalletSummary } from '@/services/botWalletApi';
import type { SharingStatus } from '@/types/network';
import type { BotWalletSummary } from '@/types/botWallet';

const STATUS_DOT: Record<SharingStatus, string> = {
  active:   'bg-emerald-500',
  idle:     'bg-gray-400',
  disabled: 'bg-gray-300',
  cooldown: 'bg-amber-500',
};

export function SidebarBalanceWidget() {
  const navigate = useNavigate();
  const { state } = useSidebar();
  const isCollapsed = state === 'collapsed';

  const aiCoin = useNetworkStore((s) => s.aiCoinBalance);
  const sharingStatus = useNetworkStore((s) => s.sharingStatus);
  const syncFromBackend = useNetworkStore((s) => s.syncFromBackend);

  const [usdBalance, setUsdBalance] = useState<number | null>(null);
  const [botSummary, setBotSummary] = useState<BotWalletSummary | null>(null);

  // Init network store on mount
  useEffect(() => {
    syncFromBackend().catch(() => {});
  }, [syncFromBackend]);

  // Poll USD balance every 60s (same pattern as UserBalanceBar)
  useEffect(() => {
    let mounted = true;
    function fetchUsd() {
      getBalance()
        .then((r) => { if (mounted) setUsdBalance(r.available / 100); })
        .catch(() => {});
    }
    fetchUsd();
    getBotWalletSummary().then(setBotSummary).catch(() => {});
    const timer = setInterval(fetchUsd, 60_000);
    return () => { mounted = false; clearInterval(timer); };
  }, []);

  const aiDisplay = aiCoin >= 1000 ? `${(aiCoin / 1000).toFixed(1)}k` : aiCoin.toFixed(1);
  const usdDisplay = usdBalance !== null ? `$${usdBalance.toFixed(2)}` : '--';
  const tooltipText = `${aiDisplay} AI | ${usdDisplay} USD`;

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <SidebarMenuButton
          className="h-9 text-[13px] cursor-pointer"
          tooltip={isCollapsed ? tooltipText : undefined}
          onClick={() => navigate('/network')}
        >
          {/* AI coin icon */}
          <div className="flex aspect-square size-6 items-center justify-center rounded-full bg-primary/10 text-primary text-[10px] font-bold shrink-0 relative">
            AI
            <span className={`absolute -bottom-0.5 -right-0.5 size-2 rounded-full border border-sidebar ${STATUS_DOT[sharingStatus]}`} />
          </div>

          {/* Balances */}
          <div className="grid flex-1 text-left text-sm leading-tight">
            <span className="truncate text-xs font-medium tabular-nums">
              {aiDisplay} <span className="text-muted-foreground">AI</span>
              <span className="text-muted-foreground mx-1">|</span>
              {usdDisplay}
            </span>
            <span className="truncate text-[10px] text-muted-foreground">
              {botSummary && botSummary.totalBots > 0
                ? `${botSummary.totalBots} bot${botSummary.totalBots > 1 ? 's' : ''} funded`
                : sharingStatus === 'active' ? 'Sharing capacity' : 'Network balance'}
            </span>
          </div>
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
