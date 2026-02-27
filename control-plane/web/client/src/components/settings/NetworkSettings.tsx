/**
 * NetworkSettings — capacity sharing, wallet, and earnings settings.
 *
 * Accessed via /settings/network. Follows the same Card/Switch/Label
 * pattern as PreferencesSettings.
 */

import { useNavigate } from 'react-router-dom';
import { useNetworkStore } from '@/stores/networkStore';
import { WalletConnect } from '@/components/network/WalletConnect';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { SharingMode } from '@/types/network';

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const HOURS = Array.from({ length: 24 }, (_, i) => `${i}:00`);

export function NetworkSettings() {
  const navigate = useNavigate();
  const config = useNetworkStore((s) => s.sharingConfig);
  const earnings = useNetworkStore((s) => s.earnings);
  const setSharingEnabled = useNetworkStore((s) => s.setSharingEnabled);
  const setSharingMode = useNetworkStore((s) => s.setSharingMode);
  const setIdleThreshold = useNetworkStore((s) => s.setIdleThreshold);
  const setMaxCapacity = useNetworkStore((s) => s.setMaxCapacity);
  const setSharingSchedule = useNetworkStore((s) => s.setSharingSchedule);

  function toggleDay(day: number) {
    const current = config.schedule?.days ?? [];
    const next = current.includes(day) ? current.filter((d) => d !== day) : [...current, day].sort();
    setSharingSchedule({
      days: next,
      startHour: config.schedule?.startHour ?? 0,
      endHour: config.schedule?.endHour ?? 23,
      timezone: config.schedule?.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone,
    });
  }

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h2 className="text-lg font-semibold tracking-tight">Network Settings</h2>
        <p className="text-sm text-muted-foreground">
          Configure AI capacity sharing, wallet, and earnings.
        </p>
      </div>

      {/* Capacity Sharing */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Capacity Sharing</CardTitle>
          <CardDescription>
            Share your unused AI/LLM capacity with the Hanzo network and earn AI coin.
            When you're not using your bots, your idle capacity is automatically shared.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Enable toggle */}
          <div className="flex items-center justify-between">
            <Label htmlFor="sharing-enabled" className="flex flex-col gap-0.5">
              <span>Enable capacity sharing</span>
              <span className="text-xs text-muted-foreground font-normal">
                Opt-in by default. Disable to stop sharing.
              </span>
            </Label>
            <Switch
              id="sharing-enabled"
              checked={config.enabled}
              onCheckedChange={setSharingEnabled}
            />
          </div>

          {config.enabled && (
            <>
              {/* Mode */}
              <div className="space-y-2">
                <Label>Sharing mode</Label>
                <Select value={config.mode} onValueChange={(v) => setSharingMode(v as SharingMode)}>
                  <SelectTrigger className="w-48">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="auto">Automatic (idle-based)</SelectItem>
                    <SelectItem value="manual">Manual (toggle on/off)</SelectItem>
                    <SelectItem value="scheduled">Scheduled (time-based)</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {/* Idle threshold — auto mode only */}
              {config.mode === 'auto' && (
                <div className="space-y-2">
                  <Label>Idle threshold: {config.idleThresholdMinutes} minutes</Label>
                  <input
                    type="range"
                    min={15}
                    max={120}
                    step={15}
                    value={config.idleThresholdMinutes}
                    onChange={(e) => setIdleThreshold(parseInt(e.target.value))}
                    className="w-full accent-primary"
                  />
                  <p className="text-xs text-muted-foreground">
                    Sharing activates after this many minutes of inactivity.
                  </p>
                </div>
              )}

              {/* Schedule — scheduled mode only */}
              {config.mode === 'scheduled' && (
                <div className="space-y-3">
                  <Label>Active days</Label>
                  <div className="flex gap-1.5">
                    {DAYS.map((name, idx) => (
                      <Button
                        key={idx}
                        variant={(config.schedule?.days ?? []).includes(idx) ? 'default' : 'outline'}
                        size="sm"
                        className="h-7 w-10 text-xs"
                        onClick={() => toggleDay(idx)}
                      >
                        {name}
                      </Button>
                    ))}
                  </div>
                  <div className="flex gap-4">
                    <div className="space-y-1">
                      <Label className="text-xs">Start</Label>
                      <Select
                        value={String(config.schedule?.startHour ?? 0)}
                        onValueChange={(v) =>
                          setSharingSchedule({
                            ...(config.schedule ?? { days: [1, 2, 3, 4, 5], endHour: 23, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone }),
                            startHour: parseInt(v),
                          })
                        }
                      >
                        <SelectTrigger className="w-24"><SelectValue /></SelectTrigger>
                        <SelectContent>
                          {HOURS.map((h, i) => <SelectItem key={i} value={String(i)}>{h}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <Label className="text-xs">End</Label>
                      <Select
                        value={String(config.schedule?.endHour ?? 23)}
                        onValueChange={(v) =>
                          setSharingSchedule({
                            ...(config.schedule ?? { days: [1, 2, 3, 4, 5], startHour: 0, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone }),
                            endHour: parseInt(v),
                          })
                        }
                      >
                        <SelectTrigger className="w-24"><SelectValue /></SelectTrigger>
                        <SelectContent>
                          {HOURS.map((h, i) => <SelectItem key={i} value={String(i)}>{h}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
              )}

              {/* Max capacity */}
              <div className="space-y-2">
                <Label>Max capacity: {config.maxCapacityPercent}%</Label>
                <input
                  type="range"
                  min={10}
                  max={100}
                  step={10}
                  value={config.maxCapacityPercent}
                  onChange={(e) => setMaxCapacity(parseInt(e.target.value))}
                  className="w-full accent-primary"
                />
                <p className="text-xs text-muted-foreground">
                  Maximum percentage of your idle capacity to share.
                </p>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Wallet */}
      <WalletConnect />

      {/* Earnings Overview */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Earnings Overview</CardTitle>
          <CardDescription>AI coin earned from sharing capacity.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div className="space-y-0.5">
              <p className="text-xs text-muted-foreground">Today</p>
              <p className="text-lg font-semibold tabular-nums">{earnings.todayEarned.toFixed(2)}</p>
            </div>
            <div className="space-y-0.5">
              <p className="text-xs text-muted-foreground">This Week</p>
              <p className="text-lg font-semibold tabular-nums">{earnings.weekEarned.toFixed(2)}</p>
            </div>
            <div className="space-y-0.5">
              <p className="text-xs text-muted-foreground">This Month</p>
              <p className="text-lg font-semibold tabular-nums">{earnings.monthEarned.toFixed(2)}</p>
            </div>
            <div className="space-y-0.5">
              <p className="text-xs text-muted-foreground">All-Time</p>
              <p className="text-lg font-semibold tabular-nums">{earnings.totalEarned.toFixed(2)}</p>
            </div>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">
              {earnings.totalHoursShared.toFixed(0)} hours shared
            </span>
            <Button variant="link" size="sm" className="h-auto p-0" onClick={() => navigate('/network')}>
              View full dashboard
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
