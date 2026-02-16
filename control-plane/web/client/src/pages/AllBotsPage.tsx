import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  SegmentedControl,
  type SegmentedControlOption,
} from "@/components/ui/segmented-control";
import {
  Grid,
  List,
  Renew,
  Terminal,
  Wifi,
  WifiOff,
} from "@/components/ui/icon-bridge";
import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CompactBotsStats } from "../components/bots/CompactBotsStats";
import { PageHeader } from "../components/PageHeader";
import { EmptyBotsState } from "../components/bots/EmptyBotsState";
import { BotGrid } from "../components/bots/BotGrid";
import { SearchFilters } from "../components/bots/SearchFilters";
import { useNodeEventsSSE, useUnifiedStatusSSE } from "../hooks/useSSE";
import { botsApi, BotsApiError } from "../services/botsApi";
import type {
  BotFilters,
  BotsResponse,
  BotWithNode,
} from "../types/bots";

type ViewMode = "grid" | "table";
const VIEW_OPTIONS: ReadonlyArray<SegmentedControlOption> = [
  { value: "grid", label: "Grid", icon: Grid },
  { value: "table", label: "Table", icon: List },
] as const;

export function AllBotsPage() {
  const navigate = useNavigate();
  const [data, setData] = useState<BotsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<BotFilters>({
    status: "online", // Default to online instead of all
    limit: 50,
    offset: 0,
  });
  const [viewMode, setViewMode] = useState<ViewMode>("grid");
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());
  const [sseConnected, setSseConnected] = useState(false);
  const [sseError, setSseError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  // Add unified status SSE for enhanced status updates
  const nodeEventsSSE = useNodeEventsSSE();
  const { latestEvent: nodeEvent } = nodeEventsSSE;

  const unifiedStatusSSE = useUnifiedStatusSSE();
  const { latestEvent: unifiedStatusEvent } = unifiedStatusSSE;

  const fetchBots = useCallback(
    async (currentFilters: BotFilters) => {
      try {
        console.log("🔄 fetchBots called with filters:", currentFilters);
        console.trace("🔍 fetchBots call stack:");
        setLoading(true);
        setError(null);
        const response = await botsApi.getAllBots(currentFilters);
        setData(response);
        setLastRefresh(new Date());
        console.log("✅ fetchBots completed successfully");
      } catch (err) {
        console.error("❌ fetchBots failed:", err);
        if (err instanceof BotsApiError) {
          // Handle specific cases where filtering returns empty results
          if (
            err.status === 404 ||
            (err.message && err.message.includes("no bots found"))
          ) {
            // Set empty data instead of error for empty filter results
            setData({
              bots: [],
              total: 0,
              online_count: 0,
              offline_count: 0,
              nodes_count: 0,
            });
            setError(null);
          } else {
            setError(err.message);
          }
        } else {
          setError("An unexpected error occurred while fetching bots");
        }
        console.error("Failed to fetch bots:", err);
      } finally {
        setLoading(false);
      }
    },
    [] // No dependencies needed
  );

  // Handle filter changes - this will trigger data fetch
  useEffect(() => {
    console.log("🔄 useEffect triggered for filters change:", filters);
    console.trace("🔍 useEffect call stack:");
    fetchBots(filters);
  }, [
    filters.status,
    filters.limit,
    filters.offset,
    filters.search,
  ]); // Remove fetchBots from dependencies to prevent infinite loops

  // SSE connection setup - for status monitoring only, no auto-refresh
  useEffect(() => {
    const setupSSE = () => {
      try {
        console.log("🔄 Setting up SSE connection for bots...");
        setSseError(null);

        const eventSource = botsApi.createEventStream(
          (event) => {
            console.log("📡 SSE Event received:", event);
            // Handle different event types
            switch (event.type) {
              case "connected":
                setSseConnected(true);
                console.log("✅ Bot SSE connected successfully");
                break;
              case "heartbeat":
                console.log("💓 SSE Heartbeat received - connection alive");
                // Keep connection alive, no action needed
                break;
              case "bot_online":
              case "bot_offline":
              case "bot_updated":
              case "bot_status_changed":
              case "node_status_changed":
              case "bots_refresh":
                // Log events but don't auto-refresh to prevent scroll jumping
                console.log("📡 Received bot update event:", event);
                console.log("ℹ️ Auto-refresh disabled - use refresh button for latest data");
                break;
              default:
                console.log(
                  "📡 Received unknown event type:",
                  event.type,
                  event
                );
            }
          },
          (error) => {
            console.error("❌ SSE Error occurred:", error);
            setSseConnected(false);
            setSseError(error.message);
            console.log(
              "⚠️ SSE connection failed, will rely on manual refresh"
            );
          },
          () => {
            console.log("✅ SSE connection established successfully");
            setSseConnected(true);
            setSseError(null);
          }
        );

        eventSourceRef.current = eventSource;
        console.log("📡 SSE EventSource created:", eventSource);
      } catch (error) {
        console.error("❌ Failed to setup SSE:", error);
        setSseError("Failed to establish real-time connection");
        console.log("⚠️ SSE setup failed, will rely on manual refresh only");
      }
    };

    // Setup SSE connection
    setupSSE();

    // Cleanup on unmount
    return () => {
      console.log("🧹 Cleaning up SSE connection...");
      if (eventSourceRef.current) {
        botsApi.closeEventStream(eventSourceRef.current);
        eventSourceRef.current = null;
        setSseConnected(false);
      }
    };
  }, []); // Only run once on mount

  // Handle unified status events for node status changes
  useEffect(() => {
    if (!nodeEvent && !unifiedStatusEvent) return;

    const event = unifiedStatusEvent || nodeEvent;
    if (!event) return;

    console.log('🔄 AllBotsPage: Received status event:', event.type, event);

    // Handle events that might affect bot status (since bots depend on nodes)
    switch (event.type) {
      case 'node_unified_status_changed':
      case 'node_state_transition':
      case 'node_status_updated':
      case 'node_health_changed':
      case 'node_online':
      case 'node_offline':
        console.log('📡 Node status changed, bots may be affected');
        // Note: We don't auto-refresh to prevent scroll jumping
        // Users can manually refresh to see updated bot status
        break;

      case 'bulk_status_update':
        console.log('📡 Bulk status update received');
        // Could trigger a refresh if many nodes are affected
        break;

      default:
        // Handle other events as needed
        break;
    }
  }, [nodeEvent, unifiedStatusEvent]);

  // Listen for custom event to clear filters from empty state
  useEffect(() => {
    const handleClearFilters = () => {
      setFilters({
        status: "all",
        limit: 50,
        offset: 0,
      });
    };

    window.addEventListener("clearBotFilters", handleClearFilters);
    return () =>
      window.removeEventListener("clearBotFilters", handleClearFilters);
  }, []);

  const handleFiltersChange = (newFilters: BotFilters) => {
    setFilters({ ...newFilters, offset: 0 }); // Reset pagination when filters change
  };

  const handleBotClick = (bot: BotWithNode) => {
    // Navigate to bot detail page using React Router
    // bot_id already contains the full format: "node_id.bot_name"
    navigate(`/bots/${encodeURIComponent(bot.bot_id)}`);
  };

  const handleRefresh = () => {
    fetchBots(filters);
  };

  const handleClearFilters = () => {
    setFilters({
      status: "online",
      limit: 50,
      offset: 0,
    });
  };

  const handleShowAll = () => {
    setFilters({
      status: "all",
      limit: 50,
      offset: 0,
    });
  };

  // Determine empty state type
  const getEmptyStateType = () => {
    if (!data) return null;

    if (data.total === 0) return "no-bots";
    if (filters.status === "online" && data.online_count === 0)
      return "no-online";
    if (filters.status === "offline" && data.offline_count === 0)
      return "no-offline";
    if (filters.search && data.bots.length === 0)
      return "no-search-results";
    if (data.bots.length === 0 && data.total > 0)
      return "no-search-results";

    return null;
  };

  // Safe data with defaults
  const safeData = data || {
    bots: [],
    total: 0,
    online_count: 0,
    offline_count: 0,
    nodes_count: 0,
  };

  return (
    <div className="space-y-8">
      <PageHeader
        title="All Bots"
        description="Browse and execute bots across all connected agent nodes."
        aside={
          <div className="flex flex-wrap items-center gap-4">
            <SegmentedControl
              value={viewMode}
              onValueChange={(mode) => setViewMode(mode as ViewMode)}
              options={VIEW_OPTIONS}
              size="sm"
              optionClassName="min-w-[90px]"
              hideLabel
            />
            <Badge
              variant={sseConnected ? "success" : "failed"}
              size="sm"
              showIcon={false}
              className="flex items-center gap-1"
            >
              {sseConnected ? <Wifi size={12} /> : <WifiOff size={12} />}
              {sseConnected ? "Live Updates" : "Disconnected"}
            </Badge>
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefresh}
              disabled={loading}
              className="flex items-center gap-2"
            >
              <Renew size={14} className={loading ? "animate-spin" : ""} />
              Refresh
            </Button>
          </div>
        }
      />

      {/* Compact Stats Summary - Always show with safe data */}
      <CompactBotsStats
        total={safeData.total}
        onlineCount={safeData.online_count}
        offlineCount={safeData.offline_count}
        nodesCount={safeData.nodes_count}
        lastRefresh={lastRefresh}
        loading={loading}
        onRefresh={handleRefresh}
      />

      {/* Search and Filters - Always show with safe data */}
      <SearchFilters
        filters={filters}
        onFiltersChange={handleFiltersChange}
        totalCount={safeData.total}
        onlineCount={safeData.online_count}
        offlineCount={safeData.offline_count}
      />

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive">
          <Terminal className="h-4 w-4" />
          <AlertTitle>Connection Error</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* SSE Error Alert */}
      {sseError && (
        <Alert variant="destructive">
          <WifiOff className="h-4 w-4" />
          <AlertTitle>Real-time Connection Error</AlertTitle>
          <AlertDescription>
            {sseError}. Data may not update automatically. Use the refresh
            button to get the latest information.
          </AlertDescription>
        </Alert>
      )}

      {/* Content Area */}
      {(() => {
        const emptyStateType = getEmptyStateType();

        if (loading && !data) {
          // Initial loading state
          return (
            <BotGrid
              bots={[]}
              loading={true}
              onBotClick={handleBotClick}
              viewMode={viewMode}
            />
          );
        }

        if (emptyStateType) {
          // Show appropriate empty state
          return (
            <EmptyBotsState
              type={emptyStateType}
              searchTerm={filters.search}
              onRefresh={handleRefresh}
              onClearFilters={handleClearFilters}
              onShowAll={handleShowAll}
              loading={loading}
            />
          );
        }

        // Show bots grid/table
        return (
          <BotGrid
            bots={safeData.bots}
            loading={loading}
            onBotClick={handleBotClick}
            viewMode={viewMode}
          />
        );
      })()}

      {/* Load More Button (if needed for pagination) */}
      {data && data.bots.length < data.total && (
        <div className="flex justify-center mt-8">
          <Button
            variant="outline"
            onClick={() => {
              const newOffset = (filters.offset || 0) + (filters.limit || 50);
              setFilters({ ...filters, offset: newOffset });
            }}
            disabled={loading}
            className="flex items-center gap-2"
          >
            {loading ? (
              <>
                <Renew size={14} className="animate-spin" />
                Loading...
              </>
            ) : (
              <>
                Load More
                <svg
                  className="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
              </>
            )}
          </Button>
        </div>
      )}

      {/* Footer Info */}
      {!loading && !error && data && data.bots.length > 0 && (
        <div className="text-center text-body-small py-4">
          Last updated: {lastRefresh.toLocaleTimeString()}
        </div>
      )}
    </div>
  );
}
