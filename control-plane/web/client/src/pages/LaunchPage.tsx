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
  Plus,
} from "@/components/ui/icon-bridge";
import {
  cloudProvision,
  cloudListNodes,
  cloudGetPresets,
  cloudDeprovision,
  type CloudPreset,
  type CloudNode,
  type CloudProvisionParams,
} from "../services/gatewayApi";

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

function formatRelativeTime(dateStr: string): string {
  const diffMs = Date.now() - new Date(dateStr).getTime();
  if (diffMs < 0) return "just now";
  const s = Math.floor(diffMs / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

export function LaunchPage() {
  const navigate = useNavigate();
  const [presets, setPresets] = useState<PresetCard[]>(DEFAULT_PRESETS);
  const [nodes, setNodes] = useState<CloudNode[]>([]);
  const [launching, setLaunching] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

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
            memoryGB: p.memory_gb,
            centsPerHour: p.cents_per_hour,
          }))
        );
      }
    } catch {
      // Use defaults
    }
  }, []);

  const loadNodes = useCallback(async () => {
    try {
      const result = await cloudListNodes();
      setNodes(result.nodes ?? []);
    } catch {
      // Ignore — may not have cloud provisioning enabled
    }
  }, []);

  useEffect(() => {
    loadPresets();
    loadNodes();
  }, [loadPresets, loadNodes]);

  const handleLaunch = async (preset: PresetCard) => {
    setLaunching(preset.id);
    setError(null);
    try {
      const params: CloudProvisionParams = {
        display_name: `${preset.name} Agent`,
        model: "claude-sonnet-4-20250514",
        provider: "digitalocean",
        instance_type: preset.slug,
        cpu: `${preset.vcpus * 1000}m`,
        memory: `${preset.memoryGB * 1024}Mi`,
      };
      await cloudProvision(params);
      await loadNodes();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Launch failed");
    } finally {
      setLaunching(null);
    }
  };

  const handleTerminate = async (nodeId: string) => {
    try {
      await cloudDeprovision(nodeId);
      await loadNodes();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Terminate failed");
    }
  };

  return (
    <div className="space-y-8">
      <PageHeader
        title="Launch an Agent"
        description="Deploy an AI agent to the cloud in seconds. Pick a spec, hit launch."
      />

      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Spec Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {presets.map((preset) => (
          <div
            key={preset.id}
            className="group relative rounded-xl border border-border bg-card p-6 transition-all hover:border-primary/50 hover:shadow-md"
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
            </div>

            <Button
              className="w-full"
              onClick={() => handleLaunch(preset)}
              disabled={launching !== null}
            >
              {launching === preset.id ? (
                <span className="flex items-center gap-2">
                  <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                  Launching...
                </span>
              ) : (
                <span className="flex items-center gap-2">
                  <Launch size={16} />
                  Launch
                </span>
              )}
            </Button>
          </div>
        ))}
      </div>

      {/* Connect Your Own */}
      <div className="rounded-xl border border-dashed border-border bg-card/50 p-6">
        <div className="flex items-start gap-4">
          <div className="rounded-lg bg-primary/10 p-3">
            <Terminal size={24} className="text-primary" />
          </div>
          <div>
            <h3 className="text-base font-semibold">
              Connect your own machine
            </h3>
            <p className="mt-1 text-sm text-muted-foreground">
              Run your bot locally and connect it to the cloud gateway.
            </p>
            <code className="mt-3 inline-block rounded-md bg-muted px-3 py-2 font-mono text-sm">
              npx @hanzo/bot
            </code>
          </div>
        </div>
      </div>

      {/* Running Agents */}
      {nodes.length > 0 && (
        <div>
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold">Running Agents</h2>
            <Button variant="ghost" size="sm" onClick={loadNodes}>
              Refresh
            </Button>
          </div>

          <div className="space-y-2">
            {nodes.map((node) => (
              <div
                key={node.node_id}
                className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3"
              >
                <div className="flex items-center gap-3">
                  <div
                    className={`h-2 w-2 rounded-full ${
                      node.status === "Running"
                        ? "bg-green-500"
                        : node.status === "Pending"
                          ? "bg-yellow-500 animate-pulse"
                          : "bg-muted-foreground"
                    }`}
                  />
                  <div>
                    <span className="font-medium">{node.node_id}</span>
                    <span className="ml-2 text-xs text-muted-foreground">
                      {node.image} · {node.os}
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <Badge
                    variant={
                      node.status === "Running" ? "default" : "secondary"
                    }
                  >
                    {node.status}
                  </Badge>
                  <span className="text-xs text-muted-foreground">
                    {formatRelativeTime(node.created_at)}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate(`/nodes/${node.node_id}`)}
                  >
                    <Monitor size={14} />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive hover:text-destructive"
                    onClick={() => handleTerminate(node.node_id)}
                  >
                    ✕
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
