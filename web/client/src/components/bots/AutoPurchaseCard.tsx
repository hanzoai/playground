/**
 * AutoPurchaseCard — manage a bot's auto-purchase rules for autonomous compute buying.
 */

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { CapacityTypeIcon, capacityTypeLabel } from '@/components/marketplace/CapacityTypeIcon';
import { AutoPurchaseRuleDialog } from './AutoPurchaseRuleDialog';
import { cn } from '@/lib/utils';
import type { AutoPurchaseRule } from '@/types/botWallet';

interface AutoPurchaseCardProps {
  botId: string;
  rules: AutoPurchaseRule[];
  walletBalance: number;
  onSave: (rule: Partial<AutoPurchaseRule>) => Promise<void>;
  onDelete: (ruleId: string) => Promise<void>;
  onExecute: (ruleId: string) => Promise<unknown>;
  className?: string;
}

export function AutoPurchaseCard({
  rules,
  walletBalance,
  onSave,
  onDelete,
  onExecute,
  className,
}: AutoPurchaseCardProps) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<AutoPurchaseRule | null>(null);
  const [executing, setExecuting] = useState<string | null>(null);

  function handleAdd() {
    setEditingRule(null);
    setDialogOpen(true);
  }

  function handleEdit(rule: AutoPurchaseRule) {
    setEditingRule(rule);
    setDialogOpen(true);
  }

  async function handleExecute(ruleId: string) {
    setExecuting(ruleId);
    try {
      await onExecute(ruleId);
    } finally {
      setExecuting(null);
    }
  }

  // Empty state
  if (rules.length === 0) {
    return (
      <>
        <Card className={cn('card-elevated', className)}>
          <CardContent className="flex items-center justify-between p-5">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-xl bg-muted/30 flex items-center justify-center text-lg">
                ⚡
              </div>
              <div>
                <p className="text-body font-medium">Auto-Purchase Compute</p>
                <p className="text-body-small text-text-tertiary">
                  Let this bot buy marketplace capacity automatically when needed
                </p>
              </div>
            </div>
            <Button variant="outline" size="sm" onClick={handleAdd}>
              Add Rule
            </Button>
          </CardContent>
        </Card>
        <AutoPurchaseRuleDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          rule={null}
          onSave={onSave}
          onDelete={async () => {}}
        />
      </>
    );
  }

  return (
    <>
      <Card className={cn('card-elevated', className)}>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-sm flex items-center gap-2">
              ⚡ Auto-Purchase Rules
            </CardTitle>
            <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={handleAdd}>
              + Add
            </Button>
          </div>
        </CardHeader>

        <CardContent className="space-y-2">
          {rules.map((rule) => {
            const canTrigger = walletBalance >= rule.minBalanceTrigger;
            return (
              <div
                key={rule.id}
                className={cn(
                  'flex items-center justify-between rounded-lg border border-border/50 p-3 transition-colors',
                  !rule.enabled && 'opacity-50',
                )}
              >
                <div className="flex items-center gap-3 min-w-0">
                  <CapacityTypeIcon type={rule.capacityType} size={16} />
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium truncate">
                        {capacityTypeLabel(rule.capacityType)}
                      </p>
                      {!rule.enabled && <Badge variant="secondary" className="text-[8px]">Paused</Badge>}
                      {rule.enabled && !canTrigger && (
                        <Badge variant="outline" className="text-[8px] text-amber-600">Low balance</Badge>
                      )}
                    </div>
                    <p className="text-[10px] text-muted-foreground truncate">
                      {rule.preferredProvider && `${rule.preferredProvider} · `}
                      {rule.preferredModel && `${rule.preferredModel} · `}
                      Max ${(rule.maxCentsPerUnit / 100).toFixed(2)}/unit · Qty {rule.defaultQuantity}
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-1.5 shrink-0">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 text-[10px] px-2"
                    onClick={() => handleEdit(rule)}
                  >
                    Edit
                  </Button>
                  {rule.enabled && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-6 text-[10px] px-2"
                      disabled={!canTrigger || executing === rule.id}
                      onClick={() => handleExecute(rule.id)}
                    >
                      {executing === rule.id ? '...' : 'Buy Now'}
                    </Button>
                  )}
                </div>
              </div>
            );
          })}
        </CardContent>
      </Card>

      <AutoPurchaseRuleDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        rule={editingRule}
        onSave={onSave}
        onDelete={async () => {
          if (editingRule) await onDelete(editingRule.id);
        }}
      />
    </>
  );
}
