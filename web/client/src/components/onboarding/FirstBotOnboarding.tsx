import { useState } from 'react';
import { DeployLocalStep } from './DeployLocalStep';
import { DeployCLIStep } from './DeployCLIStep';
import { DeployCloudStep } from './DeployCloudStep';
import { ConnectExistingStep } from './ConnectExistingStep';
import { TeamLauncher } from './TeamLauncher';

type Method = 'local' | 'cli' | 'cloud' | 'connect';

const methods: { key: Method; label: string; desc: string }[] = [
  { key: 'local', label: 'Run locally', desc: 'Desktop app or CLI on your machine' },
  { key: 'cli', label: 'Use CLI', desc: 'Headless terminal bot' },
  { key: 'cloud', label: 'Deploy to cloud', desc: 'Provision into org DOKS cluster' },
  { key: 'connect', label: 'Connect existing', desc: 'Register a running node endpoint' },
];

export function FirstBotOnboarding() {
  const [selected, setSelected] = useState<Method | null>(null);
  const [teamOpen, setTeamOpen] = useState(false);

  return (
    <div className="flex flex-col items-center gap-6 text-center max-w-md mx-auto">
      <div>
        <h2 className="text-lg font-semibold mb-1">Deploy your first bot</h2>
        <p className="text-sm text-muted-foreground">
          Choose how you'd like to get started. You can always add more nodes later.
        </p>
      </div>

      {!selected ? (
        <>
          {/* Quick launch team */}
          <button
            onClick={() => setTeamOpen(true)}
            className="w-full border border-primary/30 rounded-lg p-4 text-left hover:border-primary/60 hover:bg-primary/5 transition-colors"
          >
            <div className="flex items-center gap-2 mb-1">
              <span className="text-base leading-none">{'\u{1F680}'}</span>
              <span className="text-sm font-semibold text-primary">Launch AI Team</span>
            </div>
            <span className="text-xs text-muted-foreground">
              Deploy a pre-configured team of Hanzo AI agents instantly
            </span>
          </button>

          <div className="flex items-center gap-3 w-full text-xs text-muted-foreground/60">
            <div className="flex-1 border-t border-border/30" />
            <span>or deploy a single bot</span>
            <div className="flex-1 border-t border-border/30" />
          </div>

          <div className="grid grid-cols-2 gap-3 w-full">
            {methods.map((m) => (
              <button
                key={m.key}
                onClick={() => setSelected(m.key)}
                className="border rounded-lg p-4 text-left hover:border-primary/50 hover:bg-primary/5 transition-colors"
              >
                <span className="text-sm font-medium block mb-0.5">{m.label}</span>
                <span className="text-xs text-muted-foreground">{m.desc}</span>
              </button>
            ))}
          </div>
        </>
      ) : (
        <div className="w-full text-left">
          <button
            onClick={() => setSelected(null)}
            className="text-xs text-muted-foreground hover:text-foreground mb-4 inline-flex items-center gap-1"
          >
            &larr; Back to options
          </button>
          {selected === 'local' && <DeployLocalStep />}
          {selected === 'cli' && <DeployCLIStep />}
          {selected === 'cloud' && <DeployCloudStep />}
          {selected === 'connect' && <ConnectExistingStep />}
        </div>
      )}

      <TeamLauncher open={teamOpen} onClose={() => setTeamOpen(false)} />
    </div>
  );
}
