import React from 'react';
import type { BotDefinition } from '../types/playground';
import { Badge } from '@/components/ui/badge';
import { WatsonxAi } from '@/components/ui/icon-bridge';

interface BotsListProps {
  bots: BotDefinition[];
}

const BotsList: React.FC<BotsListProps> = ({ bots }) => {
  if (!bots || bots.length === 0) {
    return (
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <WatsonxAi className="h-4 w-4 text-muted-foreground" />
          <h4 className="text-sm font-medium">Bots (0)</h4>
        </div>
        <p className="text-body-small">No bots available.</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <WatsonxAi className="h-4 w-4 text-muted-foreground" />
        <h4 className="text-sm font-medium">Bots ({bots.length})</h4>
      </div>
      <div className="flex flex-wrap gap-2">
        {bots.map((bot) => (
          <div
            key={bot.id}
            className="min-w-[140px] rounded-lg border border-border-secondary bg-card px-3 py-2"
          >
            <div className="text-xs font-medium text-text-primary">
              {bot.id}
            </div>
            {bot.tags && bot.tags.length > 0 ? (
              <div className="mt-1 flex flex-wrap gap-1">
                {bot.tags.slice(0, 3).map((tag) => (
                  <Badge
                    key={`${bot.id}-${tag}`}
                    variant="outline"
                    className="text-[10px] bg-background text-text-tertiary border-border-secondary"
                  >
                    #{tag}
                  </Badge>
                ))}
                {bot.tags.length > 3 && (
                  <Badge
                    variant="outline"
                    className="text-[10px] bg-background text-text-quaternary border-border-secondary"
                  >
                    +{bot.tags.length - 3}
                  </Badge>
                )}
              </div>
            ) : (
              <p className="mt-1 text-[11px] text-text-tertiary">No tags</p>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default BotsList;
