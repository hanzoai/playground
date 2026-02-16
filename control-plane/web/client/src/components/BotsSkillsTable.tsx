import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { SearchBar } from "@/components/ui/SearchBar";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Function,
  Tools,
  Copy,
  Identification,
} from "@/components/ui/icon-bridge";
import { cn } from "@/lib/utils";
import type { BotDefinition, SkillDefinition } from "@/types/playground";
import type { BotDIDInfo, SkillDIDInfo } from "@/types/did";

interface BotsSkillsTableProps {
  bots: BotDefinition[];
  skills: SkillDefinition[];
  botDIDs?: Record<string, BotDIDInfo>;
  skillDIDs?: Record<string, SkillDIDInfo>;
  agentDID?: string;
  agentStatus?: {
    health_status: string;
    lifecycle_status: string;
  };
  nodeId?: string;
  className?: string;
}

interface TableItem {
  id: string;
  name: string;
  type: "bot" | "skill" | "agent";
  did?: string;
  status: "active" | "inactive" | "error";
  exposure_level?: string;
  capabilities?: string[];
  tags?: string[];
  memory_retention?: string;
}

export function BotsSkillsTable({
  bots,
  skills,
  botDIDs = {},
  skillDIDs = {},
  agentDID,
  agentStatus,
  nodeId,
  className,
}: BotsSkillsTableProps) {
  const [copyFeedback, setCopyFeedback] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState<string>("");
  const navigate = useNavigate();

  // Determine if agent is active based on health and lifecycle status
  const isAgentActive = agentStatus
    ? agentStatus.health_status === "active" &&
      (agentStatus.lifecycle_status === "ready" || agentStatus.lifecycle_status === "degraded")
    : false;

  const componentStatus: "active" | "inactive" = isAgentActive ? "active" : "inactive";

  // Combine agent DID, bots and skills into a unified table format
  const tableItems: TableItem[] = [
    // Add agent DID as first row if available
    ...(agentDID ? [{
      id: "agent",
      name: "Agent Identity",
      type: "agent" as const,
      did: agentDID,
      status: componentStatus,
    }] : []),
    ...bots.map((bot): TableItem => ({
      id: bot.id,
      name: bot.id,
      type: "bot",
      did: botDIDs[bot.id]?.did,
      status: componentStatus,
      exposure_level: botDIDs[bot.id]?.exposure_level,
      capabilities: botDIDs[bot.id]?.capabilities,
      memory_retention: bot.memory_config?.memory_retention,
      tags: bot.tags,
    })),
    ...skills.map((skill): TableItem => ({
      id: skill.id,
      name: skill.id,
      type: "skill",
      did: skillDIDs[skill.id]?.did,
      status: componentStatus,
      exposure_level: skillDIDs[skill.id]?.exposure_level,
      tags: skillDIDs[skill.id]?.tags,
    })),
  ];

  const handleCopyDID = async (did: string, name: string) => {
    try {
      await navigator.clipboard.writeText(did);
      setCopyFeedback(`${name} DID copied!`);
      setTimeout(() => setCopyFeedback(null), 2000);
    } catch (error) {
      console.error("Failed to copy DID:", error);
    }
  };

  const handleBotClick = (botId: string) => {
    if (nodeId) {
      const fullBotId = `${nodeId}.${botId}`;
      navigate(`/bots/${fullBotId}`);
    }
  };

  const handleRowClick = (item: TableItem, event: React.MouseEvent) => {
    // Prevent navigation if clicking on interactive elements like buttons
    if ((event.target as HTMLElement).closest('button')) {
      return;
    }

    if (item.type === "bot" && nodeId) {
      handleBotClick(item.id);
    }
  };

  const getStatusDot = (status: string) => {
    switch (status) {
      case "active":
        return <div className="w-2 h-2 rounded-full bg-green-500" />;
      case "inactive":
        return <div className="w-2 h-2 rounded-full bg-gray-400" />;
      case "error":
        return <div className="w-2 h-2 rounded-full bg-red-500" />;
      default:
        return <div className="w-2 h-2 rounded-full bg-gray-400" />;
    }
  };

  const getTypeIcon = (type: "bot" | "skill" | "agent") => {
    switch (type) {
      case "bot":
        return <Function className="w-4 h-4 text-accent-primary" />;
      case "skill":
        return <Tools className="w-4 h-4 text-accent-secondary" />;
      case "agent":
        return <Identification className="w-4 h-4 text-blue-500" />;
      default:
        return <Function className="w-4 h-4 text-accent-primary" />;
    }
  };

  const formatDID = (did: string, maxLength = 20) => {
    if (did.length <= maxLength) {
      return did;
    }
    const start = did.substring(0, Math.floor(maxLength / 2) - 2);
    const end = did.substring(did.length - Math.floor(maxLength / 2) + 2);
    return `${start}...${end}`;
  };

  const filteredItems = tableItems.filter(item =>
    item.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    item.id.toLowerCase().includes(searchTerm.toLowerCase())
  );

  if (filteredItems.length === 0 && searchTerm === "") {
    return (
      <Card className={cn("w-full", className)}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Function className="w-5 h-5 text-muted-foreground" />
            Bots & Skills
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <div className="flex items-center justify-center gap-2 mb-2">
              <Function className="w-8 h-8 opacity-50" />
              <Tools className="w-8 h-8 opacity-50" />
            </div>
            <p className="text-sm">No bots or skills available</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className={cn("w-full", className)}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Function className="w-5 h-5 text-muted-foreground" />
            Bots & Skills
            <Badge variant="outline" className="ml-2 text-xs">
              {filteredItems.length}
            </Badge>
          </CardTitle>
          <div className="flex items-center gap-3">
            <SearchBar
              value={searchTerm}
              onChange={setSearchTerm}
              placeholder="Search bots and skills..."
              size="sm"
              wrapperClassName="w-64"
              inputClassName="border border-border bg-background focus-visible:ring-1 focus-visible:ring-ring focus-visible:border-ring"
              clearButtonAriaLabel="Clear bot and skill search"
            />
            {copyFeedback && (
              <div className="text-sm text-status-success bg-status-success-bg border border-status-success-border rounded-md px-3 py-1">
                {copyFeedback}
              </div>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow className="border-border-secondary">
              <TableHead className="w-12">Type</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>DID</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Details</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredItems.map((item) => (
              <TableRow
                key={`${item.type}-${item.id}`}
                className={cn(
                  "border-border-secondary hover:bg-bg-hover transition-colors duration-150",
                  item.type === "bot" && nodeId && "cursor-pointer"
                )}
                onClick={(event) => handleRowClick(item, event)}
                title={item.type === "bot" && nodeId ? `Click to view ${item.name} details` : undefined}
              >
                <TableCell>
                  <div className="flex items-center justify-center">
                    {getTypeIcon(item.type)}
                  </div>
                </TableCell>

                <TableCell>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-text-primary">
                      {item.name}
                    </span>
                    <Badge
                      variant="outline"
                      className={cn(
                        "text-xs",
                        item.type === "bot"
                          ? "bg-accent-primary/10 text-accent-primary border-accent-primary/20"
                          : item.type === "skill"
                          ? "bg-accent-secondary/10 text-accent-secondary border-accent-secondary/20"
                          : "bg-blue-500/10 text-blue-500 border-blue-500/20"
                      )}
                    >
                      {item.type}
                    </Badge>
                  </div>
                </TableCell>

                <TableCell>
                  {item.did ? (
                    <div className="flex items-center gap-2 group">
                      <div className="flex items-center gap-1.5 bg-muted text-muted-foreground border border-border font-mono text-xs px-2 py-1 rounded-md">
                        <Identification className="w-3 h-3 text-accent-primary" />
                        <span title={item.did}>{formatDID(item.did, 16)}</span>
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleCopyDID(item.did!, item.name)}
                        className="h-6 w-6 p-0 opacity-0 group-hover:opacity-100 transition-opacity duration-150"
                        title="Copy DID"
                      >
                        <Copy className="w-3 h-3" />
                      </Button>
                    </div>
                  ) : (
                    <span className="text-text-quaternary text-xs">No DID</span>
                  )}
                </TableCell>

                <TableCell>
                  <div className="flex items-center gap-1.5">
                    {getStatusDot(item.status)}
                    <span className="text-body capitalize">
                      {item.status}
                    </span>
                  </div>
                </TableCell>

                <TableCell>
                  <div className="space-y-1">
                    {item.exposure_level && (
                      <Badge
                        variant="outline"
                        className="text-xs bg-card text-text-tertiary border-border-secondary"
                      >
                        {item.exposure_level}
                      </Badge>
                    )}

                    {item.memory_retention && (
                      <div className="text-xs text-text-tertiary">
                        Memory: {item.memory_retention}
                      </div>
                    )}

                    {item.capabilities && item.capabilities.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {item.capabilities.slice(0, 2).map((capability, index) => (
                          <Badge
                            key={index}
                            variant="outline"
                            className="text-xs bg-card text-text-tertiary border-border-secondary"
                          >
                            {capability}
                          </Badge>
                        ))}
                        {item.capabilities.length > 2 && (
                          <Badge
                            variant="outline"
                            className="text-xs bg-card text-text-quaternary border-border-secondary"
                          >
                            +{item.capabilities.length - 2}
                          </Badge>
                        )}
                      </div>
                    )}

                    {item.tags && item.tags.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {item.tags.slice(0, 2).map((tag, index) => (
                          <Badge
                            key={index}
                            variant="outline"
                            className="text-xs bg-card text-text-tertiary border-border-secondary"
                          >
                            #{tag}
                          </Badge>
                        ))}
                        {item.tags.length > 2 && (
                          <Badge
                            variant="outline"
                            className="text-xs bg-card text-text-quaternary border-border-secondary"
                          >
                            +{item.tags.length - 2}
                          </Badge>
                        )}
                      </div>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
