import { useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Launch,
  Terminal,
  Monitor,
  Cpu,
} from "@/components/ui/icon-bridge";
import {
  cloudProvision,
  cloudGetPresets,
  cloudGetBillingBalance,
  InsufficientFundsError,
  type CloudPreset,
  type CloudProvisionParams,
  type CloudBillingBalance,
} from "../services/gatewayApi";
import { TOP_UP_URL } from "../services/billingApi";

interface PresetCard {
  id: string;
  name: string;
  description: string;
  slug: string;
  vcpus: number;
  memoryGB: number;
  centsPerHour: number;
}

const DEFAULT_PRESETS: PresetCard[] = [
  {
    id: "starter",
    name: "Starter",
    description: "Light tasks, chat bots, simple automations",
    slug: "s-1vcpu-2gb",
    vcpus: 1,
    memoryGB: 2,
    centsPerHour: 2,
  },
  {
    id: "pro",
    name: "Pro",
    description: "Code generation, research, multi-tool agents",
    slug: "s-2vcpu-4gb",
    vcpus: 2,
    memoryGB: 4,
    centsPerHour: 4,
  },
  {
    id: "power",
    name: "Power",
    description: "Heavy workloads, browser automation, large projects",
    slug: "s-4vcpu-8gb",
    vcpus: 4,
    memoryGB: 8,
    centsPerHour: 7,
  },
  {
    id: "gpu",
    name: "GPU",
    description: "ML training, image generation, video processing",
    slug: "g-2vcpu-8gb",
    vcpus: 2,
    memoryGB: 8,
    centsPerHour: 7,
  },
];

function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export function LaunchPage() {
  const navigate = useNavigate();
  const [presets, setPresets] = useState<PresetCard[]>(DEFAULT_PRESETS);
  const [launching, setLaunching] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [billingError, setBillingError] = useState<{
    balanceCents: number;
    requiredCents: number;
  } | null>(null);
  const [balance, setBalance] = useState<CloudBillingBalance | null>(null);

  const loadPresets = useCallback(async () => {
    try {
      const result = await cloudGetPresets();
      if (result.presets?.length > 0) {
        setPresets(
          result.presets.map((p: CloudPreset) => ({
            id: p.id,
            name: p.name,
            description: p.description,
            slug: p.slug,
            vcpus: p.vcpus,
            memoryGB: p.memory_gb ?? p.memoryGB ?? 2,
            centsPerHour: p.cents_per_hour ?? p.centsPerHour ?? 0,
          }))
        );
      }
    } catch {
      // Use defaults
    }
  }, []);

  const loadBalance = useCallback(async () => {
    try {
      const result = await cloudGetBillingBalance();
      setBalance(result);
    } catch {
      // Balance check is optional — don't block the page
    }
  }, []);

  useEffect(() => {
    loadPresets();
    loadBalance();
  }, [loadPresets, loadBalance]);

  /** Hours the user can afford for a given preset, or null if balance unknown. */
  const hoursAffordable = (presetId: string): number | null => {
    if (!balance?.presets?.length) return null;
    const match = balance.presets.find((p) => p.name === presetId);
    return match ? match.hours_afford : null;
  };

  const handleLaunch = async (preset: PresetCard) => {
    setLaunching(preset.id);
    setError(null);
    setBillingError(null);
    try {
      const params: CloudProvisionParams = {
        display_name: `${preset.name} Bot`,
        model: "claude-sonnet-4-20250514",
        provider: "digitalocean",
        instance_type: preset.slug,
        cpu: `${preset.vcpus * 1000}m`,
        memory: `${preset.memoryGB * 1024}Mi`,
      };
      await cloudProvision(params);
      navigate("/nodes");
    } catch (err) {
      if (err instanceof InsufficientFundsError) {
        setBillingError({
          balanceCents: err.balanceCents,
          requiredCents: err.requiredCents,
        });
        loadBalance();
      } else {
        setError(err instanceof Error ? err.message : "Launch failed");
      }
    } finally {
      setLaunching(null);
    }
  };

  return (
    <div className="space-y-8">
      <PageHeader
        title="Launch a Bot"
        description="Deploy a bot to the cloud in seconds. Pick a spec, hit launch."
      />

      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {billingError && (
        <div className="rounded-lg border border-amber-500/50 bg-amber-500/10 p-4 text-sm space-y-2">
          <p className="font-semibold text-amber-600 dark:text-amber-400">
            Insufficient funds to launch this bot.
          </p>
          <p className="text-muted-foreground">
            Required: {formatCents(billingError.requiredCents)} (1 hour minimum).
            Your balance: {formatCents(billingError.balanceCents)}.
          </p>
          <a
            href={TOP_UP_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-block rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            Add Funds
          </a>
        </div>
      )}

      {/* Balance display */}
      {balance && balance.balance_cents > 0 && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Balance: <span className="font-mono font-semibold text-foreground">{formatCents(balance.balance_cents)}</span></span>
        </div>
      )}

      {/* Spec Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {presets.map((preset) => {
          const hours = hoursAffordable(preset.id);
          const canAfford = hours === null || hours >= 1;

          return (
            <div
              key={preset.id}
              className={`group relative rounded-xl border bg-card p-6 transition-all ${
                canAfford
                  ? "border-border hover:border-primary/50 hover:shadow-md"
                  : "border-border/50 opacity-60"
              }`}
            >
              <div className="mb-4 flex items-center justify-between">
                <h3 className="text-lg font-semibold">{preset.name}</h3>
                <Badge variant="secondary" className="font-mono text-xs">
                  {formatCents(preset.centsPerHour)}/hr
                </Badge>
              </div>

              <p className="mb-4 text-sm text-muted-foreground">
                {preset.description}
              </p>

              <div className="mb-6 flex gap-4 text-xs text-muted-foreground">
                <span className="flex items-center gap-1">
                  <Cpu size={12} />
                  {preset.vcpus} vCPU{preset.vcpus > 1 ? "s" : ""}
                </span>
                <span className="flex items-center gap-1">
                  <Monitor size={12} />
                  {preset.memoryGB}GB RAM
                </span>
                {hours !== null && (
                  <span className="ml-auto font-mono">
                    {hours}h
                  </span>
                )}
              </div>

              <Button
                className="w-full"
                onClick={() => handleLaunch(preset)}
                disabled={launching !== null || !canAfford}
              >
                {launching === preset.id ? (
                  <span className="flex items-center gap-2">
                    <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                    Launching...
                  </span>
                ) : !canAfford ? (
                  <span className="flex items-center gap-2">
                    Insufficient Funds
                  </span>
                ) : (
                  <span className="flex items-center gap-2">
                    <Launch size={16} />
                    Launch
                  </span>
                )}
              </Button>
            </div>
          );
        })}
      </div>

      {/* Connect Your Own */}
      <div className="rounded-xl border border-dashed border-border bg-card/50 p-4 sm:p-6">
        <div className="flex flex-col sm:flex-row items-start gap-3 sm:gap-4">
          <div className="rounded-lg bg-primary/10 p-3 shrink-0">
            <Terminal size={24} className="text-primary" />
          </div>
          <div className="min-w-0 w-full">
            <h3 className="text-base font-semibold">
              Connect your own machine
            </h3>
            <p className="mt-1 text-sm text-muted-foreground">
              Run your bot locally and connect it to the cloud gateway.
            </p>
            <code className="mt-3 inline-block rounded-md bg-muted px-3 py-2 font-mono text-xs sm:text-sm break-all">
              npx @hanzo/bot
            </code>
          </div>
        </div>
      </div>

    </div>
  );
}
