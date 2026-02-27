import {
  Activity,
  Analytics,
  Chat,
  CheckmarkFilled,
  Code,
  Copy,
  InProgress,
  Play,
  Time,
  View,
} from "../components/ui/icon-bridge";
import { BotBudgetCard } from "../components/bots/BotBudgetCard";
import { BotWalletCard } from "../components/bots/BotWalletCard";
import { AutoPurchaseCard } from "../components/bots/AutoPurchaseCard";
import { useBotBudget } from "../hooks/useBotBudget";
import { useBotWallet } from "../hooks/useBotWallet";
import { useNetworkStore } from "../stores/networkStore";
import { getBalance } from "../services/billingApi";
import { useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { useParams } from "react-router-dom";
import { DIDIdentityBadge } from "../components/did/DIDDisplay";
import { Badge } from "../components/ui/badge";
import { ExecutionForm } from "../components/bots/ExecutionForm";
import { ExecutionHistoryList } from "../components/bots/ExecutionHistoryList";
import {
  ExecutionQueue,
  type ExecutionQueueRef,
  type QueuedExecution,
} from "../components/bots/ExecutionQueue";
import { FormattedOutput } from "../components/bots/FormattedOutput";
import { PerformanceChart } from "../components/bots/PerformanceChart";
import { StatusIndicator } from "../components/bots/StatusIndicator";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "../components/ui/tabs";
import { SegmentedControl } from "../components/ui/segmented-control";
import type { SegmentedControlOption } from "../components/ui/segmented-control";
import { ResponsiveGrid } from "@/components/layout/ResponsiveGrid";
import { botsApi } from "../services/botsApi";
import { normalizeExecutionStatus } from "../utils/status";
import type { ExecutionHistory, PerformanceMetrics } from "../types/execution";
import type { BotWithNode } from "../types/bots";
import { generateExampleData, validateFormData } from "../utils/schemaUtils";

const RESULT_VIEW_OPTIONS: ReadonlyArray<SegmentedControlOption> = [
  { value: "formatted", label: "Formatted", icon: View },
  { value: "json", label: "JSON", icon: Code },
] as const;

export function BotDetailPage() {
  const { fullBotId } = useParams<{ fullBotId: string }>();

  const [bot, setBot] = useState<BotWithNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Multiple execution state
  const [selectedExecution, setSelectedExecution] =
    useState<QueuedExecution | null>(null);
  const [resultViewMode, setResultViewMode] = useState<"formatted" | "json">(
    "formatted"
  );
  const executionQueueRef = useRef<ExecutionQueueRef | null>(null);
  const chatIframeRef = useRef<HTMLIFrameElement | null>(null);
  const { apiKey } = useAuth();
  const [activeView, setActiveView] = useState<"execute" | "chat">("execute");

  // History and metrics
  const [history, setHistory] = useState<ExecutionHistory | null>(null);
  const [metrics, setMetrics] = useState<PerformanceMetrics | null>(null);

  // Form state
  const [formData, setFormData] = useState<any>({});
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [isExecuting, setIsExecuting] = useState(false);

  // Budget
  const { budget, status: budgetStatus, spendHistory, saveBudget, removeBudget } = useBotBudget(fullBotId);

  // Wallet
  const botWallet = useBotWallet(fullBotId);
  const aiCoinBalance = useNetworkStore((s) => s.aiCoinBalance);
  const [usdBalanceCents, setUsdBalanceCents] = useState(0);
  useEffect(() => {
    getBalance().then((r) => setUsdBalanceCents(r.available)).catch(() => {});
  }, []);

  const gatewayOrigin = (import.meta.env.VITE_BOT_GATEWAY_URL as string) || "https://gw.hanzo.bot";

  // Send IAM token to embedded Bot Control UI via postMessage
  const handleIframeLoad = useCallback(() => {
    const iframe = chatIframeRef.current;
    if (!iframe?.contentWindow || !apiKey) return;
    iframe.contentWindow.postMessage(
      { type: "hanzo:iam-token", token: apiKey },
      gatewayOrigin,
    );
  }, [apiKey, gatewayOrigin]);

  useEffect(() => {
    loadBotDetails();
    loadMetrics();
    loadHistory();
  }, [fullBotId]);

  const loadBotDetails = async () => {
    if (!fullBotId) return;

    try {
      setLoading(true);
      const data = await botsApi.getBotDetails(fullBotId);
      setBot(data);

      // Initialize form with example data if schema is available
      if (data.input_schema) {
        const exampleData = generateExampleData(data.input_schema);
        setFormData({ input: exampleData });
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load bot details"
      );
    } finally {
      setLoading(false);
    }
  };

  const loadMetrics = async () => {
    if (!fullBotId) return;
    try {
      const data = await botsApi.getPerformanceMetrics(fullBotId);
      setMetrics(data);
    } catch (err) {
      console.error("Failed to load metrics:", err);
    }
  };

  const loadHistory = async () => {
    if (!fullBotId) return;
    try {
      const data = await botsApi.getExecutionHistory(
        fullBotId,
        1,
        10
      );
      setHistory(data);
    } catch (err) {
      console.error("Failed to load history:", err);
    }
  };

  const handleExecute = () => {
    if (!bot || !fullBotId || !executionQueueRef.current) return;

    // Validate form data
    if (bot.input_schema) {
      const validation = validateFormData(
        formData.input,
        bot.input_schema
      );
      if (!validation.isValid) {
        setValidationErrors(validation.errors);
        return;
      }
    }

    setValidationErrors([]);
    setIsExecuting(true);

    // Add execution to queue
    const executionId = executionQueueRef.current.addExecution(
      formData.input || {}
    );
    console.log("Added execution to queue:", executionId);

    // Reset executing state after a brief delay
    setTimeout(() => setIsExecuting(false), 500);
  };

  const handleExecutionComplete = (execution: QueuedExecution) => {
    // Refresh history and metrics after execution
    loadHistory();
    loadMetrics();

    // Auto-select the completed execution to show results
    setSelectedExecution(execution);
  };

  const handleExecutionSelect = (execution: QueuedExecution | null) => {
    // Handle selection from the execution queue
    setSelectedExecution(execution);
  };

  const handleCopyCommand = () => {
    if (!bot || !fullBotId) return;

    const baseUrl = window.location.origin;
    const curlCommand = `curl -X POST ${baseUrl}/api/v1/execute/${encodeURIComponent(
      fullBotId
    )} \\
  -H "Content-Type: application/json" \\
  -d '${JSON.stringify({ input: formData.input || {} }, null, 2)}'`;

    navigator.clipboard.writeText(curlCommand);
  };

  const toggleResultView = () => {
    setResultViewMode((prev) => (prev === "formatted" ? "json" : "formatted"));
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="flex flex-col items-center gap-3">
          <InProgress className="h-8 w-8 animate-spin text-muted-foreground" />
          <p className="text-body-small">
            Loading bot details...
          </p>
        </div>
      </div>
    );
  }

  if (error || !bot) {
    return (
      <div className="container mx-auto px-4 py-8">
        <Alert className="max-w-md mx-auto border-red-200 bg-red-50">
          <div className="flex items-center gap-2">
            <div className="h-4 w-4 rounded-full bg-red-500" />
            <div>
              <h3 className="font-semibold text-red-900">Error</h3>
              <p className="text-sm text-red-700">
                {error || "Bot not found"}
              </p>
            </div>
          </div>
        </Alert>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-display">
              {bot.name}
            </h2>
            <p className="text-body">
              {bot.description || "No description available"}
            </p>
          </div>
          <div className="flex items-center gap-4">
            <StatusIndicator
              status={
                bot.node_status === "active"
                  ? "online"
                  : bot.node_status === "inactive"
                  ? "offline"
                  : "unknown"
              }
            />
            <div className="flex items-center gap-2">
              <Button
                variant={activeView === "execute" ? "default" : "outline"}
                size="sm"
                onClick={() => setActiveView("execute")}
                className="flex items-center gap-2"
              >
                <Play className="h-4 w-4" />
                Execute
              </Button>
              <Button
                variant={activeView === "chat" ? "default" : "outline"}
                size="sm"
                onClick={() => setActiveView("chat")}
                className="flex items-center gap-2"
              >
                <Chat className="h-4 w-4" />
                Live Chat
              </Button>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleCopyCommand}
              className="flex items-center gap-2"
            >
              <Copy className="h-4 w-4" />
              Copy cURL
            </Button>
          </div>
        </div>
        <div className="flex items-center gap-4 text-body">
          <span>Node: {bot.node_id}</span>
          <span>•</span>
          <span>ID: {fullBotId}</span>
          <span>•</span>
          <DIDIdentityBadge nodeId={bot.node_id} showDID={true} />
        </div>
        {bot.tags && bot.tags.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-2">
            {bot.tags.map((tag) => (
              <Badge
                key={`${bot.bot_id}-${tag}`}
                variant="secondary"
                className="text-xs"
              >
                #{tag}
              </Badge>
            ))}
          </div>
        )}
      </div>

      {/* Chat View — Bot Control UI iframe */}
      {activeView === "chat" && (
        <Card className="card-elevated">
          <CardContent className="p-0">
            <iframe
              ref={chatIframeRef}
              src={`${gatewayOrigin}/?token=${encodeURIComponent(apiKey || "")}`}
              onLoad={handleIframeLoad}
              style={{ width: "100%", height: "calc(100vh - 240px)", border: "none" }}
              allow="clipboard-write"
              title="Bot Control UI"
            />
          </CardContent>
        </Card>
      )}

      {/* Execute View */}
      {activeView === "execute" && <>

      {/* Quick Stats */}
      {metrics && (
        <ResponsiveGrid preset="quarters" gap="md" align="start">
          <Card className="card-elevated">
            <CardContent className="p-4">
              <div className="flex items-center gap-2 mb-2">
                <Time className="h-4 w-4 text-text-tertiary" />
                <span className="text-caption">Avg Response</span>
              </div>
              <p className="text-heading-3">{metrics.avg_response_time_ms}ms</p>
            </CardContent>
          </Card>

          <Card className="card-elevated">
            <CardContent className="p-4">
              <div className="flex items-center gap-2 mb-2">
                <CheckmarkFilled className="h-4 w-4 text-status-success" />
                <span className="text-caption">Success Rate</span>
              </div>
              <p className="text-heading-3">
                {(metrics.success_rate * 100).toFixed(1)}%
              </p>
            </CardContent>
          </Card>

          <Card className="card-elevated">
            <CardContent className="p-4">
              <div className="flex items-center gap-2 mb-2">
                <Analytics className="h-4 w-4 text-text-tertiary" />
                <span className="text-caption">Total Executions</span>
              </div>
              <p className="text-heading-3">{metrics.total_executions}</p>
            </CardContent>
          </Card>

          <Card className="card-elevated">
            <CardContent className="p-4">
              <div className="flex items-center gap-2 mb-2">
                <Activity className="h-4 w-4 text-text-tertiary" />
                <span className="text-caption">Last 24h</span>
              </div>
              <p className="text-heading-3">{metrics.executions_last_24h}</p>
            </CardContent>
          </Card>
        </ResponsiveGrid>
      )}

      {/* Budget */}
      <BotBudgetCard
        botId={fullBotId!}
        budget={budget}
        status={budgetStatus}
        spendHistory={spendHistory}
        onSave={saveBudget}
        onDelete={removeBudget}
      />

      {/* Wallet */}
      <BotWalletCard
        botId={fullBotId!}
        wallet={botWallet.wallet}
        transactions={botWallet.transactions}
        userAiCoinBalance={aiCoinBalance}
        userUsdBalanceCents={usdBalanceCents}
        onFund={botWallet.fundWallet}
        onWithdraw={botWallet.withdrawFromWallet}
      />

      {/* Auto-Purchase */}
      <AutoPurchaseCard
        botId={fullBotId!}
        rules={botWallet.autoPurchaseRules}
        walletBalance={botWallet.wallet?.aiCoinBalance ?? 0}
        onSave={botWallet.saveAutoPurchaseRule}
        onDelete={botWallet.removeAutoPurchaseRule}
        onExecute={botWallet.triggerAutoPurchase}
      />

      {/* Responsive Layout */}
      <ResponsiveGrid columns={{ base: 1, lg: 12 }} gap="md" align="start">
        {/* Left Panel - Input & Configuration */}
        <div className="lg:col-span-5 space-y-6">
          {/* Input Form */}
          <Card className="card-elevated">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Play className="h-5 w-5" />
                Execute Bot
              </CardTitle>
              <CardDescription>
                Provide input data and execute the bot
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <ExecutionForm
                schema={bot.input_schema}
                formData={formData}
                onChange={setFormData}
                validationErrors={validationErrors}
              />

              <Button
                onClick={handleExecute}
                className="w-full"
                size="lg"
                disabled={isExecuting}
              >
                {isExecuting ? (
                  <>
                    <InProgress className="h-4 w-4 mr-2 animate-spin" />
                    Executing...
                  </>
                ) : (
                  <>
                    <Play className="h-4 w-4 mr-2" />
                    Execute Bot
                  </>
                )}
              </Button>
            </CardContent>
          </Card>

          {/* Schema Information */}
          <Tabs defaultValue="input" className="space-y-4">
            <TabsList variant="underline" className="grid w-full grid-cols-2">
              <TabsTrigger value="input" variant="underline">Input Schema</TabsTrigger>
              <TabsTrigger value="output" variant="underline">Output Schema</TabsTrigger>
            </TabsList>

            <TabsContent value="input">
              <Card className="card-elevated">
                <CardHeader>
                  <CardTitle className="text-sm">Input Schema</CardTitle>
                  <CardDescription>
                    Expected input format for this bot
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {bot.input_schema ? (
                    <pre className="bg-bg-secondary p-4 rounded-lg text-sm overflow-auto scrollbar-thin border border-border-secondary">
                      {JSON.stringify(bot.input_schema, null, 2)}
                    </pre>
                  ) : (
                    <p className="text-text-tertiary">
                      No input schema available
                    </p>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="output">
              <Card className="card-elevated">
                <CardHeader>
                  <CardTitle className="text-sm">Output Schema</CardTitle>
                  <CardDescription>
                    Expected output format from this bot
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {bot.output_schema ? (
                    <pre className="bg-bg-secondary p-4 rounded-lg text-sm overflow-auto scrollbar-thin border border-border-secondary">
                      {JSON.stringify(bot.output_schema, null, 2)}
                    </pre>
                  ) : (
                    <p className="text-text-tertiary">
                      No output schema available
                    </p>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>

        {/* Right Panel - Execution & Results */}
        <div className="lg:col-span-7 space-y-6">
          {/* Execution Queue */}
          <ExecutionQueue
            botId={fullBotId!}
            onExecutionComplete={handleExecutionComplete}
            onExecutionSelect={handleExecutionSelect}
            ref={executionQueueRef}
          />

          {/* Selected Execution Result */}
          {selectedExecution && (
            <Card className="card-elevated">
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle>Execution Result</CardTitle>
                    <CardDescription>
                      Result from execution: {selectedExecution.inputSummary}
                    </CardDescription>
                  </div>
                  {normalizeExecutionStatus(selectedExecution.status) === "succeeded" &&
                    selectedExecution.result && (
                      <SegmentedControl
                        value={resultViewMode}
                        onValueChange={(mode) =>
                          setResultViewMode(mode as typeof resultViewMode)
                        }
                        options={RESULT_VIEW_OPTIONS}
                        size="sm"
                        optionClassName="min-w-[120px]"
                      />
                    )}
                </div>
              </CardHeader>
              <CardContent>
                {normalizeExecutionStatus(selectedExecution.status) === "succeeded" &&
                selectedExecution.result ? (
                  <FormattedOutput
                    data={selectedExecution.result}
                    showRaw={resultViewMode === "json"}
                    onToggleView={toggleResultView}
                    executionId={selectedExecution.id}
                    duration={selectedExecution.duration}
                    status={normalizeExecutionStatus(selectedExecution.status)}
                    hideHeader={true}
                  />
                ) : selectedExecution.status === "failed" ? (
                  <div className="text-center py-8">
                    <div className="h-8 w-8 mx-auto mb-3 rounded-full bg-status-error-bg flex items-center justify-center">
                      <div className="h-4 w-4 rounded-full bg-status-error" />
                    </div>
                    <p className="text-status-error font-medium mb-1">
                      Execution Failed
                    </p>
                    <p className="text-body-small">{selectedExecution.error}</p>
                  </div>
                ) : (
                  <div className="flex items-center justify-center py-8">
                    <InProgress className="h-6 w-6 animate-spin text-text-tertiary mr-3" />
                    <span className="text-text-secondary">Executing...</span>
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          {/* Activity & Performance */}
          <Tabs defaultValue="activity" className="space-y-4">
            <TabsList variant="underline" className="grid w-full grid-cols-2">
              <TabsTrigger value="activity" variant="underline" className="gap-2">
                <Activity className="h-4 w-4" />
                Activity
              </TabsTrigger>
              <TabsTrigger
                value="performance"
                variant="underline"
                className="gap-2"
              >
                <Analytics className="h-4 w-4" />
                Performance
              </TabsTrigger>
            </TabsList>

            <TabsContent value="activity">
              <Card className="card-elevated">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Activity className="h-5 w-5" />
                    Recent Executions
                  </CardTitle>
                  <CardDescription>
                    Latest execution attempts and their results
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <ExecutionHistoryList
                    history={history}
                    onLoadMore={() => {
                      // TODO: Implement pagination
                    }}
                  />
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="performance">
              <Card className="card-elevated">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Analytics className="h-5 w-5" />
                    Performance Metrics
                  </CardTitle>
                  <CardDescription>
                    Response times and success rates over time
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <PerformanceChart metrics={metrics} />
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </ResponsiveGrid>

      </>}
    </div>
  );
}
