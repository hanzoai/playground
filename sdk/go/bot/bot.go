package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hanzoai/playground/sdk/go/ai"
	"github.com/hanzoai/playground/sdk/go/client"
	"github.com/hanzoai/playground/sdk/go/types"
)

type executionContextKey struct{}

// ExecutionContext captures the headers Playground sends with each execution request.
type ExecutionContext struct {
	RunID             string
	ExecutionID       string
	ParentExecutionID string
	SessionID         string
	ActorID           string
	WorkflowID        string
	ParentWorkflowID  string
	RootWorkflowID    string
	Depth             int
	NodeID       string
	BotName      string
	StartedAt         time.Time
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// HandlerFunc processes a bot invocation.
type HandlerFunc func(ctx context.Context, input map[string]any) (any, error)

// BotOption applies metadata to a bot registration.
type BotOption func(*BotHandler)

// WithInputSchema overrides the auto-generated input schema.
func WithInputSchema(raw json.RawMessage) BotOption {
	return func(r *BotHandler) {
		if len(raw) > 0 {
			r.InputSchema = raw
		}
	}
}

// WithOutputSchema overrides the default output schema.
func WithOutputSchema(raw json.RawMessage) BotOption {
	return func(r *BotHandler) {
		if len(raw) > 0 {
			r.OutputSchema = raw
		}
	}
}

// WithCLI marks this bot as CLI-accessible.
func WithCLI() BotOption {
	return func(r *BotHandler) {
		r.CLIEnabled = true
	}
}

// WithDefaultCLI marks the bot as the default CLI handler.
func WithDefaultCLI() BotOption {
	return func(r *BotHandler) {
		r.CLIEnabled = true
		r.DefaultCLI = true
	}
}

// WithCLIFormatter registers a custom formatter for CLI output.
func WithCLIFormatter(formatter func(context.Context, any, error)) BotOption {
	return func(r *BotHandler) {
		r.CLIFormatter = formatter
	}
}

// WithDescription adds a human-readable description for help/list commands.
func WithDescription(desc string) BotOption {
	return func(r *BotHandler) {
		r.Description = desc
	}
}

// BotHandler represents a single handler exposed by the bot.
type BotHandler struct {
	Name         string
	Handler      HandlerFunc
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage

	CLIEnabled   bool
	DefaultCLI   bool
	CLIFormatter func(context.Context, any, error)
	Description  string
}

// Config drives Bot behaviour.
type Config struct {
	// NodeID is the unique identifier for this bot node. Required.
	// Must be a non-empty identifier suitable for registration (alphanumeric
	// characters, hyphens are recommended). Example: "my-bot-1".
	NodeID string

	// Version identifies the bot implementation version. Required.
	// Typically in semver or short string form (e.g. "v1.2.3" or "1.0.0").
	Version string

	// TeamID groups related bots together for organization. Optional.
	// Default: "default" (if empty, New() sets it to "default").
	TeamID string

	// PlaygroundURL is the base URL of the Playground control plane server.
	// Optional for local-only or serverless usage, required when registering
	// with a control plane or making cross-node calls. Default: empty.
	// Format: a valid HTTP/HTTPS URL, e.g. "https://playground.example.com".
	PlaygroundURL string

	// ListenAddress is the network address the bot HTTP server binds to.
	// Optional. Default: ":8001" (if empty, New() sets it to ":8001").
	// Format: "host:port" or ":port" (e.g. ":8001" or "0.0.0.0:8001").
	ListenAddress string

	// PublicURL is the public-facing base URL reported to the control plane.
	// Optional. Default: "http://localhost" + ListenAddress (if empty,
	// New() constructs a default using ListenAddress).
	// Format: a valid HTTP/HTTPS URL.
	PublicURL string

	// Token is the bearer token used for authenticating to the control plane.
	// Optional. Default: empty (no auth). When set, the token is sent as
	// an Authorization: Bearer <token> header on control-plane requests.
	Token string

	// DeploymentType describes how the bot runs (affects execution behavior).
	// Optional. Default: "long_running". Common values: "long_running",
	// "serverless". Use a descriptive string for custom modes.
	DeploymentType string

	// LeaseRefreshInterval controls how frequently the bot refreshes its
	// lease/heartbeat with the control plane. Optional.
	// Default: 2m (2 minutes). Valid: any positive time.Duration.
	LeaseRefreshInterval time.Duration

	// DisableLeaseLoop disables automatic periodic lease refreshes.
	// Optional. Default: false.
	DisableLeaseLoop bool

	// Logger is used for bot logging output. Optional.
	// Default: a standard logger writing to stdout with the "[bot] " prefix
	// (if nil, New() creates a default logger).
	Logger *log.Logger

	// AIConfig configures LLM/AI capabilities for the bot.
	// Optional. If nil, AI features are disabled. Provide a valid
	// *ai.Config to enable AI-related APIs.
	AIConfig *ai.Config

	// CLIConfig controls CLI-specific behaviour and help text.
	// Optional. If nil, CLI behavior uses sensible defaults.
	CLIConfig *CLIConfig

	// MemoryBackend allows plugging in a custom memory storage backend.
	// Optional. If nil, an in-memory backend is used (data lost on restart).
	MemoryBackend MemoryBackend
}

// CLIConfig controls CLI behaviour and presentation.
type CLIConfig struct {
	AppName        string
	AppDescription string
	DisableColors  bool

	DefaultOutputFormat string
	HelpPreamble        string
	HelpEpilog          string
	EnvironmentVars     []string
}

// Bot manages registration, lease renewal, and HTTP routing.
type Bot struct {
	cfg        Config
	client     *client.Client
	httpClient *http.Client
	bots  map[string]*BotHandler
	aiClient   *ai.Client // AI/LLM client
	memory     *Memory    // Memory system for state management

	serverMu sync.RWMutex
	server   *http.Server

	stopLease chan struct{}
	logger    *log.Logger

	router      http.Handler
	handlerOnce sync.Once

	initMu        sync.Mutex
	initialized   bool
	leaseLoopOnce sync.Once

	defaultCLIBot string
}

// envWithFallback reads HANZO_{key} first, then falls back to AGENT_{key}.
func envWithFallback(key string) string {
	if v := os.Getenv("HANZO_" + key); v != "" {
		return v
	}
	return os.Getenv("AGENT_" + key)
}

// ConfigFromEnv constructs a Config from environment variables.
// It reads HANZO_* variables first, falling back to AGENT_* for backward compatibility.
//
// Supported environment variables:
//
//	HANZO_NODE_ID / AGENT_NODE_ID       - Bot node identifier
//	HANZO_VERSION / AGENT_VERSION       - Bot version
//	HANZO_TEAM_ID / AGENT_TEAM_ID       - Team identifier
//	HANZO_SERVER_URL / AGENT_SERVER_URL  - Playground control plane URL
//	HANZO_LISTEN_ADDR / AGENT_LISTEN_ADDR - HTTP listen address
//	HANZO_PUBLIC_URL / AGENT_PUBLIC_URL  - Public-facing URL
//	HANZO_TOKEN / AGENT_TOKEN            - Bearer token
func ConfigFromEnv() Config {
	return Config{
		NodeID:        envWithFallback("NODE_ID"),
		Version:       envWithFallback("VERSION"),
		TeamID:        envWithFallback("TEAM_ID"),
		PlaygroundURL: envWithFallback("SERVER_URL"),
		ListenAddress: envWithFallback("LISTEN_ADDR"),
		PublicURL:     envWithFallback("PUBLIC_URL"),
		Token:         envWithFallback("TOKEN"),
	}
}

// New constructs a Bot.
func New(cfg Config) (*Bot, error) {
	if cfg.NodeID == "" {
		return nil, errors.New("config.NodeID is required")
	}
	if cfg.Version == "" {
		return nil, errors.New("config.Version is required")
	}
	if cfg.TeamID == "" {
		cfg.TeamID = "default"
	}
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = ":8001"
	}
	if cfg.PublicURL == "" {
		cfg.PublicURL = "http://localhost" + cfg.ListenAddress
	}
	if strings.TrimSpace(cfg.DeploymentType) == "" {
		cfg.DeploymentType = "long_running"
	}
	if cfg.LeaseRefreshInterval <= 0 {
		cfg.LeaseRefreshInterval = 2 * time.Minute
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stdout, "[bot] ", log.LstdFlags)
	}

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}

	// Initialize AI client if config provided
	var aiClient *ai.Client
	var err error
	if cfg.AIConfig != nil {
		aiClient, err = ai.NewClient(cfg.AIConfig)
		if err != nil {
			return nil, fmt.Errorf("initialize AI client: %w", err)
		}
	}

	b := &Bot{
		cfg:        cfg,
		httpClient: httpClient,
		bots:  make(map[string]*BotHandler),
		aiClient:   aiClient,
		memory:     NewMemory(cfg.MemoryBackend),
		stopLease:  make(chan struct{}),
		logger:     cfg.Logger,
	}

	if strings.TrimSpace(cfg.PlaygroundURL) != "" {
		c, err := client.New(cfg.PlaygroundURL, client.WithHTTPClient(httpClient), client.WithBearerToken(cfg.Token))
		if err != nil {
			return nil, err
		}
		b.client = c
	}

	return b, nil
}

func contextWithExecution(ctx context.Context, exec ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey{}, exec)
}

func executionContextFrom(ctx context.Context) ExecutionContext {
	if ctx == nil {
		return ExecutionContext{}
	}
	if val, ok := ctx.Value(executionContextKey{}).(ExecutionContext); ok {
		return val
	}
	return ExecutionContext{}
}

// ChildContext creates a new execution context for a nested local call.
func (ec ExecutionContext) ChildContext(agentNodeID, botName string) ExecutionContext {
	runID := ec.RunID
	if runID == "" {
		runID = ec.WorkflowID
	}
	if runID == "" {
		runID = generateRunID()
	}

	workflowID := ec.WorkflowID
	if workflowID == "" {
		workflowID = runID
	}
	rootWorkflowID := ec.RootWorkflowID
	if rootWorkflowID == "" {
		rootWorkflowID = workflowID
	}

	return ExecutionContext{
		RunID:             runID,
		ExecutionID:       generateExecutionID(),
		ParentExecutionID: ec.ExecutionID,
		SessionID:         ec.SessionID,
		ActorID:           ec.ActorID,
		WorkflowID:        workflowID,
		ParentWorkflowID:  workflowID,
		RootWorkflowID:    rootWorkflowID,
		Depth:             ec.Depth + 1,
		NodeID:       agentNodeID,
		BotName:      botName,
		StartedAt:         time.Now(),
	}
}

func generateRunID() string {
	return fmt.Sprintf("run_%d_%06d", time.Now().UnixNano(), rand.Intn(1_000_000))
}

func generateExecutionID() string {
	return fmt.Sprintf("exec_%d_%06d", time.Now().UnixNano(), rand.Intn(1_000_000))
}

func cloneInputMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	copied := make(map[string]any, len(input))
	for k, v := range input {
		copied[k] = v
	}
	return copied
}

func stringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if str, ok := val.(string); ok && strings.TrimSpace(str) != "" {
				return strings.TrimSpace(str)
			}
		}
	}
	return ""
}

func rawToMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// RegisterBot makes a handler available at /bots/{name}.
func (b *Bot) RegisterBot(name string, handler HandlerFunc, opts ...BotOption) {
	if handler == nil {
		panic("nil handler supplied")
	}

	meta := &BotHandler{
		Name:         name,
		Handler:      handler,
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":true}`),
		OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":true}`),
	}
	for _, opt := range opts {
		opt(meta)
	}

	if meta.DefaultCLI {
		if b.defaultCLIBot != "" && b.defaultCLIBot != name {
			b.logger.Printf("warn: default CLI bot already set to %s, ignoring default flag on %s", b.defaultCLIBot, name)
			meta.DefaultCLI = false
		} else {
			b.defaultCLIBot = name
		}
	}

	b.bots[name] = meta
}

// Initialize registers the bot with the Playground control plane without starting a listener.
func (b *Bot) Initialize(ctx context.Context) error {
	b.initMu.Lock()
	defer b.initMu.Unlock()

	if b.initialized {
		return nil
	}

	if b.client == nil {
		return errors.New("PlaygroundURL is required when running in server mode")
	}

	if len(b.bots) == 0 {
		return errors.New("no bots registered")
	}

	if err := b.registerNode(ctx); err != nil {
		return fmt.Errorf("register node: %w", err)
	}

	if err := b.markReady(ctx); err != nil {
		b.logger.Printf("warn: initial status update failed: %v", err)
	}

	b.startLeaseLoop()
	b.initialized = true
	return nil
}

// Run intelligently routes between CLI and server modes.
func (b *Bot) Run(ctx context.Context) error {
	args := os.Args[1:]
	if len(args) == 0 && !b.hasCLIBots() {
		return b.Serve(ctx)
	}

	if len(args) > 0 && args[0] == "serve" {
		return b.Serve(ctx)
	}

	return b.runCLI(ctx, args)
}

// Serve starts the bot HTTP server, registers with the control plane, and blocks until ctx is cancelled.
func (b *Bot) Serve(ctx context.Context) error {
	if err := b.Initialize(ctx); err != nil {
		return err
	}

	if err := b.startServer(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	// listen for shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-ctx.Done():
		return b.shutdown(context.Background())
	case sig := <-sigCh:
		b.logger.Printf("received signal %s, shutting down", sig)
		return b.shutdown(context.Background())
	}
}

func (b *Bot) registerNode(ctx context.Context) error {
	now := time.Now().UTC()

	bots := make([]types.BotDefinition, 0, len(b.bots))
	for _, bot := range b.bots {
		bots = append(bots, types.BotDefinition{
			ID:           bot.Name,
			InputSchema:  bot.InputSchema,
			OutputSchema: bot.OutputSchema,
		})
	}

	payload := types.NodeRegistrationRequest{
		ID:        b.cfg.NodeID,
		TeamID:    b.cfg.TeamID,
		BaseURL:   strings.TrimSuffix(b.cfg.PublicURL, "/"),
		Version:   b.cfg.Version,
		Bots: bots,
		Skills:    []types.SkillDefinition{},
		CommunicationConfig: types.CommunicationConfig{
			Protocols:         []string{"http"},
			HeartbeatInterval: "0s",
		},
		HealthStatus:  "healthy",
		LastHeartbeat: now,
		RegisteredAt:  now,
		Metadata: map[string]any{
			"deployment": map[string]any{
				"environment": "development",
				"platform":    "go",
			},
			"sdk": map[string]any{
				"language": "go",
			},
		},
		Features:       map[string]any{},
		DeploymentType: b.cfg.DeploymentType,
	}

	_, err := b.client.RegisterNode(ctx, payload)
	if err != nil {
		return err
	}

	b.logger.Printf("node %s registered with Playground", b.cfg.NodeID)
	return nil
}

func (b *Bot) markReady(ctx context.Context) error {
	score := 100
	_, err := b.client.UpdateStatus(ctx, b.cfg.NodeID, types.NodeStatusUpdate{
		Phase:       "ready",
		HealthScore: &score,
	})
	return err
}

func (b *Bot) startServer() error {
	server := &http.Server{
		Addr:    b.cfg.ListenAddress,
		Handler: b.Handler(),
	}
	b.serverMu.Lock()
	b.server = server
	b.serverMu.Unlock()

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			b.logger.Printf("server error: %v", err)
		}
	}()

	b.logger.Printf("listening on %s", b.cfg.ListenAddress)
	return nil
}

// Handler exposes the bot as an http.Handler for serverless or custom hosting scenarios.
func (b *Bot) Handler() http.Handler {
	return b.handler()
}

// ServeHTTP implements http.Handler directly.
func (b *Bot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.Handler().ServeHTTP(w, r)
}

// Execute runs a specific bot by name.
func (b *Bot) Execute(ctx context.Context, botName string, input map[string]any) (any, error) {
	bot, ok := b.bots[botName]
	if !ok {
		return nil, fmt.Errorf("unknown bot %q", botName)
	}
	if input == nil {
		input = make(map[string]any)
	}
	return bot.Handler(ctx, input)
}

// HandleServerlessEvent allows custom serverless entrypoints to normalize arbitrary
// platform events (Lambda, Vercel, Supabase, etc.) before delegating to the bot.
// The adapter can rewrite the incoming event into the generic payload that
// handleExecute expects: keys like path, target/bot, input, execution_context.
func (b *Bot) HandleServerlessEvent(ctx context.Context, event map[string]any, adapter func(map[string]any) map[string]any) (map[string]any, int, error) {
	if adapter != nil {
		event = adapter(event)
	}

	path := stringFromMap(event, "path", "rawPath")
	bot := stringFromMap(event, "bot", "target", "skill")
	if bot == "" && path != "" {
		cleaned := strings.Trim(path, "/")
		parts := strings.Split(cleaned, "/")
		if len(parts) >= 2 && (parts[0] == "execute" || parts[0] == "bots" || parts[0] == "skills") {
			bot = parts[1]
		} else if len(parts) == 1 {
			bot = parts[0]
		}
	}
	if bot == "" {
		return map[string]any{"error": "missing target or bot"}, http.StatusBadRequest, nil
	}

	input := extractInputFromServerless(event)
	execCtx := b.buildExecutionContextFromServerless(&http.Request{Header: http.Header{}}, event, bot)
	ctx = contextWithExecution(ctx, execCtx)

	handler, ok := b.bots[bot]
	if !ok {
		return map[string]any{"error": "bot not found"}, http.StatusNotFound, nil
	}

	result, err := handler.Handler(ctx, input)
	if err != nil {
		return map[string]any{"error": err.Error()}, http.StatusInternalServerError, nil
	}

	// Normalize to map for consistent JSON responses.
	if payload, ok := result.(map[string]any); ok {
		return payload, http.StatusOK, nil
	}
	return map[string]any{"result": result}, http.StatusOK, nil
}

func (b *Bot) handler() http.Handler {
	b.handlerOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", b.healthHandler)
		mux.HandleFunc("/discover", b.handleDiscover)
		mux.HandleFunc("/execute", b.handleExecute)
		mux.HandleFunc("/execute/", b.handleExecute)
		mux.HandleFunc("/bots/", b.handleBot)
		b.router = mux
	})
	return b.router
}

func (b *Bot) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (b *Bot) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, b.discoveryPayload())
}

func (b *Bot) discoveryPayload() map[string]any {
	bots := make([]map[string]any, 0, len(b.bots))
	for _, bot := range b.bots {
		bots = append(bots, map[string]any{
			"id":            bot.Name,
			"input_schema":  rawToMap(bot.InputSchema),
			"output_schema": rawToMap(bot.OutputSchema),
			"tags":          []string{},
		})
	}

	deployment := strings.TrimSpace(b.cfg.DeploymentType)
	if deployment == "" {
		deployment = "long_running"
	}

	return map[string]any{
		"node_id":         b.cfg.NodeID,
		"version":         b.cfg.Version,
		"deployment_type": deployment,
		"bots":       bots,
		"skills":          []map[string]any{},
	}
}

func (b *Bot) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetName := strings.TrimPrefix(r.URL.Path, "/execute")
	targetName = strings.TrimPrefix(targetName, "/")

	var payload map[string]any
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	botName := strings.TrimSpace(targetName)
	if botName == "" {
		botName = stringFromMap(payload, "bot", "target", "skill")
	}

	if botName == "" {
		http.Error(w, "missing target or bot", http.StatusBadRequest)
		return
	}

	bot, ok := b.bots[botName]
	if !ok {
		http.NotFound(w, r)
		return
	}

	input := extractInputFromServerless(payload)
	execCtx := b.buildExecutionContextFromServerless(r, payload, botName)
	ctx := contextWithExecution(r.Context(), execCtx)

	result, err := bot.Handler(ctx, input)
	if err != nil {
		b.logger.Printf("bot %s failed: %v", botName, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func extractInputFromServerless(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}

	if raw, ok := payload["input"]; ok {
		if m, ok := raw.(map[string]any); ok {
			return m
		}
		return map[string]any{"value": raw}
	}

	filtered := make(map[string]any)
	for k, v := range payload {
		switch strings.ToLower(k) {
		case "target", "bot", "skill", "type", "target_type", "path", "execution_context", "executioncontext", "context":
			continue
		default:
			filtered[k] = v
		}
	}
	return filtered
}

func (b *Bot) buildExecutionContextFromServerless(r *http.Request, payload map[string]any, botName string) ExecutionContext {
	execCtx := ExecutionContext{
		RunID:             strings.TrimSpace(r.Header.Get("X-Run-ID")),
		ExecutionID:       strings.TrimSpace(r.Header.Get("X-Execution-ID")),
		ParentExecutionID: strings.TrimSpace(r.Header.Get("X-Parent-Execution-ID")),
		SessionID:         strings.TrimSpace(r.Header.Get("X-Session-ID")),
		ActorID:           strings.TrimSpace(r.Header.Get("X-Actor-ID")),
		WorkflowID:        strings.TrimSpace(r.Header.Get("X-Workflow-ID")),
		NodeID:       b.cfg.NodeID,
		BotName:      botName,
		StartedAt:         time.Now(),
	}

	if ctxMap, ok := payload["execution_context"].(map[string]any); ok {
		if execCtx.ExecutionID == "" {
			execCtx.ExecutionID = stringFromMap(ctxMap, "execution_id", "executionId")
		}
		if execCtx.RunID == "" {
			execCtx.RunID = stringFromMap(ctxMap, "run_id", "runId")
		}
		if execCtx.WorkflowID == "" {
			execCtx.WorkflowID = stringFromMap(ctxMap, "workflow_id", "workflowId")
		}
		if execCtx.ParentExecutionID == "" {
			execCtx.ParentExecutionID = stringFromMap(ctxMap, "parent_execution_id", "parentExecutionId")
		}
		if execCtx.SessionID == "" {
			execCtx.SessionID = stringFromMap(ctxMap, "session_id", "sessionId")
		}
		if execCtx.ActorID == "" {
			execCtx.ActorID = stringFromMap(ctxMap, "actor_id", "actorId")
		}
	}

	if execCtx.RunID == "" {
		execCtx.RunID = generateRunID()
	}
	if execCtx.ExecutionID == "" {
		execCtx.ExecutionID = generateExecutionID()
	}
	if execCtx.WorkflowID == "" {
		execCtx.WorkflowID = execCtx.RunID
	}
	if execCtx.RootWorkflowID == "" {
		execCtx.RootWorkflowID = execCtx.WorkflowID
	}
	if execCtx.ParentWorkflowID == "" && execCtx.ParentExecutionID != "" {
		execCtx.ParentWorkflowID = execCtx.RootWorkflowID
	}

	return execCtx
}

func (b *Bot) handleBot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/bots/")
	if name == "" {
		http.NotFound(w, r)
		return
	}

	bot, ok := b.bots[name]
	if !ok {
		http.NotFound(w, r)
		return
	}

	defer r.Body.Close()
	var input map[string]any
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	execCtx := ExecutionContext{
		RunID:             r.Header.Get("X-Run-ID"),
		ExecutionID:       r.Header.Get("X-Execution-ID"),
		ParentExecutionID: r.Header.Get("X-Parent-Execution-ID"),
		SessionID:         r.Header.Get("X-Session-ID"),
		ActorID:           r.Header.Get("X-Actor-ID"),
		WorkflowID:        r.Header.Get("X-Workflow-ID"),
		NodeID:       b.cfg.NodeID,
		BotName:      name,
		StartedAt:         time.Now(),
	}
	if execCtx.WorkflowID == "" {
		execCtx.WorkflowID = execCtx.RunID
	}
	if execCtx.RootWorkflowID == "" {
		execCtx.RootWorkflowID = execCtx.WorkflowID
	}

	ctx := contextWithExecution(r.Context(), execCtx)

	// In serverless mode we want a synchronous execution so the control plane can return
	// the result immediately; skip the async path even if an execution ID is present.
	if b.cfg.DeploymentType != "serverless" && execCtx.ExecutionID != "" && strings.TrimSpace(b.cfg.PlaygroundURL) != "" {
		go b.executeBotAsync(bot, cloneInputMap(input), execCtx)
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":        "processing",
			"execution_id":  execCtx.ExecutionID,
			"run_id":        execCtx.RunID,
			"bot_name": name,
		})
		return
	}

	result, err := bot.Handler(ctx, input)
	if err != nil {
		b.logger.Printf("bot %s failed: %v", name, err)
		response := map[string]any{
			"error": err.Error(),
		}
		writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (b *Bot) executeBotAsync(bot *BotHandler, input map[string]any, execCtx ExecutionContext) {
	ctx := contextWithExecution(context.Background(), execCtx)
	start := time.Now()

	defer func() {
		if rec := recover(); rec != nil {
			errMsg := fmt.Sprintf("panic: %v", rec)
			payload := map[string]any{
				"status":        "failed",
				"error":         errMsg,
				"execution_id":  execCtx.ExecutionID,
				"run_id":        execCtx.RunID,
				"completed_at":  time.Now().UTC().Format(time.RFC3339),
				"duration_ms":   time.Since(start).Milliseconds(),
				"bot_name": bot.Name,
			}
			if err := b.sendExecutionStatus(execCtx.ExecutionID, payload); err != nil {
				b.logger.Printf("failed to send panic status: %v", err)
			}
		}
	}()

	result, err := bot.Handler(ctx, input)
	payload := map[string]any{
		"execution_id":  execCtx.ExecutionID,
		"run_id":        execCtx.RunID,
		"completed_at":  time.Now().UTC().Format(time.RFC3339),
		"duration_ms":   time.Since(start).Milliseconds(),
		"bot_name": bot.Name,
	}

	if err != nil {
		payload["status"] = "failed"
		payload["error"] = err.Error()
	} else {
		payload["status"] = "succeeded"
		payload["result"] = result
	}

	if err := b.sendExecutionStatus(execCtx.ExecutionID, payload); err != nil {
		b.logger.Printf("async status update failed: %v", err)
	}
}

func (b *Bot) sendExecutionStatus(executionID string, payload map[string]any) error {
	base := strings.TrimSpace(b.cfg.PlaygroundURL)
	if executionID == "" || base == "" {
		return fmt.Errorf("missing execution id or Playground URL")
	}
	callbackURL := strings.TrimSuffix(base, "/") + "/api/v1/executions/" + url.PathEscape(executionID) + "/status"
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode status payload: %w", err)
	}
	return b.postExecutionStatus(context.Background(), callbackURL, payloadBytes)
}

func (b *Bot) postExecutionStatus(ctx context.Context, callbackURL string, payload []byte) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, callbackURL, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return fmt.Errorf("create status request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := b.httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				cancel()
				return nil
			}
			lastErr = fmt.Errorf("status update returned %d", resp.StatusCode)
		}
		cancel()
		if attempt < 4 {
			time.Sleep(time.Second << attempt)
		}
	}
	return lastErr
}

// Call invokes another bot via the Playground control plane, preserving execution context.
func (b *Bot) Call(ctx context.Context, target string, input map[string]any) (map[string]any, error) {
	if strings.TrimSpace(b.cfg.PlaygroundURL) == "" {
		return nil, errors.New("PlaygroundURL is required to call other bots")
	}

	if !strings.Contains(target, ".") {
		target = fmt.Sprintf("%s.%s", b.cfg.NodeID, strings.TrimPrefix(target, "."))
	}

	execCtx := executionContextFrom(ctx)
	runID := execCtx.RunID
	if runID == "" {
		runID = generateRunID()
	}

	payload := map[string]any{"input": input}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal call payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/execute/%s", strings.TrimSuffix(b.cfg.PlaygroundURL, "/"), strings.TrimPrefix(target, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Run-ID", runID)
	if execCtx.ExecutionID != "" {
		req.Header.Set("X-Parent-Execution-ID", execCtx.ExecutionID)
	}
	if execCtx.WorkflowID != "" {
		req.Header.Set("X-Workflow-ID", execCtx.WorkflowID)
	}
	if execCtx.SessionID != "" {
		req.Header.Set("X-Session-ID", execCtx.SessionID)
	}
	if execCtx.ActorID != "" {
		req.Header.Set("X-Actor-ID", execCtx.ActorID)
	}
	if b.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform execute call: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read execute response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("execute failed: %s", strings.TrimSpace(string(bodyBytes)))
	}

	var execResp struct {
		ExecutionID  string         `json:"execution_id"`
		RunID        string         `json:"run_id"`
		Status       string         `json:"status"`
		Result       map[string]any `json:"result"`
		ErrorMessage *string        `json:"error_message"`
	}
	if err := json.Unmarshal(bodyBytes, &execResp); err != nil {
		return nil, fmt.Errorf("decode execute response: %w", err)
	}

	if execResp.ErrorMessage != nil && *execResp.ErrorMessage != "" {
		return nil, fmt.Errorf("execute error: %s", *execResp.ErrorMessage)
	}
	if !strings.EqualFold(execResp.Status, "succeeded") {
		return nil, fmt.Errorf("execute status %s", execResp.Status)
	}

	return execResp.Result, nil
}

// emitWorkflowEvent sends a workflow event to the control plane asynchronously.
// Failures are logged but do not impact the caller.
func (b *Bot) emitWorkflowEvent(
	execCtx ExecutionContext,
	status string,
	input map[string]any,
	result any,
	err error,
	durationMS int64,
) {
	if strings.TrimSpace(b.cfg.PlaygroundURL) == "" {
		return
	}

	event := types.WorkflowExecutionEvent{
		ExecutionID: execCtx.ExecutionID,
		WorkflowID:  execCtx.WorkflowID,
		RunID:       execCtx.RunID,
		BotID:  execCtx.BotName,
		Type:        execCtx.BotName,
		NodeID: b.cfg.NodeID,
		Status:      status,
	}

	if execCtx.ParentExecutionID != "" {
		event.ParentExecutionID = &execCtx.ParentExecutionID
	}
	if execCtx.ParentWorkflowID != "" {
		event.ParentWorkflowID = &execCtx.ParentWorkflowID
	}
	if input != nil {
		event.InputData = input
	}
	if result != nil {
		event.Result = result
	}
	if err != nil {
		event.Error = err.Error()
	}
	if durationMS > 0 {
		event.DurationMS = &durationMS
	}

	if sendErr := b.sendWorkflowEvent(event); sendErr != nil {
		b.logger.Printf("workflow event send failed: %v", sendErr)
	}
}

func (b *Bot) sendWorkflowEvent(event types.WorkflowExecutionEvent) error {
	url := strings.TrimSuffix(b.cfg.PlaygroundURL, "/") + "/api/v1/workflow/executions/events"

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if b.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	return nil
}

// CallLocal invokes a registered bot directly within this bot process,
// maintaining execution lineage and emitting workflow events to the control plane.
// It should be used for same-node composition; use Call for cross-node calls.
func (b *Bot) CallLocal(ctx context.Context, botName string, input map[string]any) (any, error) {
	bot, ok := b.bots[botName]
	if !ok {
		return nil, fmt.Errorf("unknown bot %q", botName)
	}

	parentCtx := executionContextFrom(ctx)

	childCtx := b.buildChildContext(parentCtx, botName)
	ctx = contextWithExecution(ctx, childCtx)

	b.emitWorkflowEvent(childCtx, "running", input, nil, nil, 0)

	start := time.Now()
	result, err := bot.Handler(ctx, input)
	durationMS := time.Since(start).Milliseconds()

	if err != nil {
		b.emitWorkflowEvent(childCtx, "failed", input, nil, err, durationMS)
	} else {
		b.emitWorkflowEvent(childCtx, "succeeded", input, result, nil, durationMS)
	}

	return result, err
}

func (b *Bot) buildChildContext(parent ExecutionContext, botName string) ExecutionContext {
	if parent.RunID == "" && parent.ExecutionID == "" {
		runID := generateRunID()
		return ExecutionContext{
			RunID:          runID,
			ExecutionID:    generateExecutionID(),
			SessionID:      parent.SessionID,
			ActorID:        parent.ActorID,
			WorkflowID:     runID,
			RootWorkflowID: runID,
			Depth:          0,
			NodeID:    b.cfg.NodeID,
			BotName:   botName,
			StartedAt:      time.Now(),
		}
	}

	return parent.ChildContext(b.cfg.NodeID, botName)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// best-effort fallback
		_, _ = w.Write([]byte(`{}`))
	}
}

func (b *Bot) startLeaseLoop() {
	if b.cfg.DisableLeaseLoop || b.cfg.LeaseRefreshInterval <= 0 {
		return
	}

	b.leaseLoopOnce.Do(func() {
		ticker := time.NewTicker(b.cfg.LeaseRefreshInterval)
		go func() {
			for {
				select {
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := b.markReady(ctx); err != nil {
						b.logger.Printf("lease refresh failed: %v", err)
					}
					cancel()
				case <-b.stopLease:
					ticker.Stop()
					return
				}
			}
		}()
	})
}

func (b *Bot) shutdown(ctx context.Context) error {
	close(b.stopLease)

	if _, err := b.client.Shutdown(ctx, b.cfg.NodeID, types.ShutdownRequest{Reason: "shutdown"}); err != nil {
		b.logger.Printf("failed to notify shutdown: %v", err)
	}

	b.serverMu.RLock()
	server := b.server
	b.serverMu.RUnlock()

	if server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}

// AI makes an AI/LLM call with the given prompt and options.
// Returns an error if AI is not configured for this bot.
//
// Example usage:
//
//	response, err := bot.AI(ctx, "What is the weather?",
//	    ai.WithSystem("You are a weather assistant"),
//	    ai.WithTemperature(0.7))
func (b *Bot) AI(ctx context.Context, prompt string, opts ...ai.Option) (*ai.Response, error) {
	if b.aiClient == nil {
		return nil, errors.New("AI not configured for this bot; set AIConfig in bot Config")
	}
	return b.aiClient.Complete(ctx, prompt, opts...)
}

// AIStream makes a streaming AI/LLM call.
// Returns channels for streaming chunks and errors.
//
// Example usage:
//
//	chunks, errs := bot.AIStream(ctx, "Tell me a story")
//	for chunk := range chunks {
//	    fmt.Print(chunk.Choices[0].Delta.Content)
//	}
//	if err := <-errs; err != nil {
//	    log.Fatal(err)
//	}
func (b *Bot) AIStream(ctx context.Context, prompt string, opts ...ai.Option) (<-chan ai.StreamChunk, <-chan error) {
	if b.aiClient == nil {
		errCh := make(chan error, 1)
		errCh <- errors.New("AI not configured for this bot; set AIConfig in bot Config")
		close(errCh)
		chunkCh := make(chan ai.StreamChunk)
		close(chunkCh)
		return chunkCh, errCh
	}
	return b.aiClient.StreamComplete(ctx, prompt, opts...)
}

// ExecutionContextFrom returns the execution context embedded in the provided context, if any.
func ExecutionContextFrom(ctx context.Context) ExecutionContext {
	return executionContextFrom(ctx)
}

// Memory returns the bot's memory system for state management.
// Memory provides hierarchical scoped storage (workflow, session, user, global).
//
// Example usage:
//
//	// Store in default (session) scope
//	bot.Memory().Set(ctx, "key", "value")
//
//	// Retrieve from session scope
//	val, _ := bot.Memory().Get(ctx, "key")
//
//	// Use global scope for cross-session data
//	bot.Memory().GlobalScope().Set(ctx, "shared_key", data)
func (b *Bot) Memory() *Memory {
	return b.memory
}
