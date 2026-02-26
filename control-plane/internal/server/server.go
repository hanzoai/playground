package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/cloud"
	"github.com/hanzoai/playground/control-plane/internal/config"
	"github.com/hanzoai/playground/control-plane/internal/core/interfaces"
	coreservices "github.com/hanzoai/playground/control-plane/internal/core/services" // Core services
	"github.com/hanzoai/playground/control-plane/internal/events"                     // Event system
	"github.com/hanzoai/playground/control-plane/internal/handlers"                   // Agent handlers
	"github.com/hanzoai/playground/control-plane/internal/handlers/ui"                // UI handlers
	"github.com/hanzoai/playground/control-plane/internal/infrastructure/communication"
	"github.com/hanzoai/playground/control-plane/internal/infrastructure/process"
	infrastorage "github.com/hanzoai/playground/control-plane/internal/infrastructure/storage"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/proxy"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
	"github.com/hanzoai/playground/control-plane/internal/services" // Services
	"github.com/hanzoai/playground/control-plane/internal/spaces"
	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/internal/utils"
	client "github.com/hanzoai/playground/control-plane/web/client"

	"github.com/gin-contrib/cors" // CORS middleware
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PlaygroundServer represents the core Playground control plane service.
type PlaygroundServer struct {
	storage               storage.StorageProvider
	cache                 storage.CacheProvider
	Router                *gin.Engine
	uiService             *services.UIService           // Add UIService
	executionsUIService   *services.ExecutionsUIService // Add ExecutionsUIService
	healthMonitor         *services.HealthMonitor
	presenceManager       *services.PresenceManager
	statusManager         *services.StatusManager // Add StatusManager for unified status management
	botService          interfaces.BotService // Add BotService for lifecycle management
	nodeClient           interfaces.NodeClient  // Add NodeClient for MCP communication
	config                *config.Config
	storageHealthOverride func(context.Context) gin.H
	cacheHealthOverride   func(context.Context) gin.H
	// DID Services
	keystoreService *services.KeystoreService
	didService      *services.DIDService
	vcService       *services.VCService
	didRegistry     *services.DIDRegistry
	playgroundHome  string
	// Cleanup service
	cleanupService        *handlers.ExecutionCleanupService
	payloadStore          services.PayloadStore
	registryWatcherCancel context.CancelFunc
	zapAdmin              *zapAdminNode
	zapAdminPort          int
	webhookDispatcher        services.WebhookDispatcher
	observabilityForwarder   services.ObservabilityForwarder
	cloudProvisioner         *cloud.Provisioner
	spaceStore               spaces.Store
}

// NewPlaygroundServer creates a new instance of the PlaygroundServer.
func NewPlaygroundServer(cfg *config.Config) (*PlaygroundServer, error) {
	// Define playgroundHome at the very top (PLAYGROUND_HOME preferred, AGENTS_HOME fallback)
	playgroundHome := os.Getenv("PLAYGROUND_HOME")
	if playgroundHome == "" {
		playgroundHome = os.Getenv("AGENTS_HOME") // Legacy fallback
	}
	if playgroundHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		playgroundHome = filepath.Join(homeDir, ".hanzo/playground")
	}

	dirs, err := utils.EnsureDataDirectories()
	if err != nil {
		return nil, fmt.Errorf("failed to ensure data directories: %w", err)
	}

	factory := &storage.StorageFactory{}
	storageProvider, cacheProvider, err := factory.CreateStorage(cfg.Storage)
	if err != nil {
		return nil, err
	}

	Router := gin.Default()

	// Sync installed.yaml to database for package visibility
	_ = SyncPackagesFromRegistry(playgroundHome, storageProvider)

	// Initialize agent client for communication with agent nodes
	nodeClient := communication.NewHTTPNodeClient(storageProvider, 5*time.Second)

	// Create infrastructure components for BotService
	fileSystem := infrastorage.NewFileSystemAdapter()
	registryPath := filepath.Join(playgroundHome, "installed.json")
	registryStorage := infrastorage.NewLocalRegistryStorage(fileSystem, registryPath)
	processManager := process.NewProcessManager()
	portManager := process.NewPortManager()

	// Create BotService
	botService := coreservices.NewBotService(processManager, portManager, registryStorage, nodeClient, playgroundHome)

	// Initialize StatusManager for unified status management
	statusManagerConfig := services.StatusManagerConfig{
		ReconcileInterval:       30 * time.Second,
		StatusCacheTTL:          5 * time.Minute,
		MaxTransitionTime:       2 * time.Minute,
		HeartbeatStaleThreshold: cfg.Agents.NodeHealth.HeartbeatStaleThreshold,
	}

	// Create UIService first (without StatusManager)
	uiService := services.NewUIService(storageProvider, nodeClient, botService, nil)

	// Create StatusManager with UIService and NodeClient
	statusManager := services.NewStatusManager(storageProvider, statusManagerConfig, uiService, nodeClient)

	// Update UIService with StatusManager reference
	uiService = services.NewUIService(storageProvider, nodeClient, botService, statusManager)

	// Presence manager tracks node leases so stale nodes age out quickly
	presenceConfig := services.PresenceManagerConfig{
		HeartbeatTTL:  5 * time.Minute,
		SweepInterval: 30 * time.Second,
		HardEvictTTL:  30 * time.Minute,
	}
	presenceManager := services.NewPresenceManager(statusManager, presenceConfig)

	executionsUIService := services.NewExecutionsUIService(storageProvider) // Initialize ExecutionsUIService

	// Initialize health monitor with configurable settings
	healthMonitorConfig := services.HealthMonitorConfig{
		CheckInterval:       cfg.Agents.NodeHealth.CheckInterval,
		CheckTimeout:        cfg.Agents.NodeHealth.CheckTimeout,
		ConsecutiveFailures: cfg.Agents.NodeHealth.ConsecutiveFailures,
		RecoveryDebounce:    cfg.Agents.NodeHealth.RecoveryDebounce,
	}
	healthMonitor := services.NewHealthMonitor(storageProvider, healthMonitorConfig, uiService, nodeClient, statusManager, presenceManager)
	presenceManager.SetExpireCallback(healthMonitor.UnregisterAgent)

	// Initialize DID services if enabled
	var keystoreService *services.KeystoreService
	var didService *services.DIDService
	var vcService *services.VCService
	var didRegistry *services.DIDRegistry

	if cfg.Features.DID.Enabled {
		fmt.Println("🔐 Initializing DID and VC services...")

		// Use universal path management for DID directories
		dirs, err := utils.EnsureDataDirectories()
		if err != nil {
			return nil, fmt.Errorf("failed to create DID directories: %w", err)
		}

		// Update keystore path to use universal paths
		if cfg.Features.DID.Keystore.Path == "./data/keys" {
			cfg.Features.DID.Keystore.Path = dirs.KeysDir
		}

		fmt.Printf("🔑 Creating keystore service at: %s\n", cfg.Features.DID.Keystore.Path)
		// Instantiate services in dependency order: Keystore → DID → VC, Registry
		keystoreService, err = services.NewKeystoreService(&cfg.Features.DID.Keystore)
		if err != nil {
			return nil, fmt.Errorf("failed to create keystore service: %w", err)
		}

		fmt.Println("📋 Creating DID registry...")
		didRegistry = services.NewDIDRegistryWithStorage(storageProvider)

		fmt.Println("🆔 Creating DID service...")
		didService = services.NewDIDService(&cfg.Features.DID, keystoreService, didRegistry)

		fmt.Println("📜 Creating VC service...")
		vcService = services.NewVCService(&cfg.Features.DID, didService, storageProvider)

		// Initialize services
		fmt.Println("🔧 Initializing DID registry...")
		if err = didRegistry.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize DID registry: %w", err)
		}

		fmt.Println("🔧 Initializing VC service...")
		if err = vcService.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize VC service: %w", err)
		}

		// Generate af server ID based on agents home directory
		agentsServerID := generatePlaygroundServerID(playgroundHome)

		// Initialize af server DID with dynamic ID
		fmt.Printf("🧠 Initializing af server DID (ID: %s)...\n", agentsServerID)
		if err := didService.Initialize(agentsServerID); err != nil {
			return nil, fmt.Errorf("failed to initialize af server DID: %w", err)
		}

		// Validate that af server DID was successfully created
		registry, err := didService.GetRegistry(agentsServerID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate af server DID creation: %w", err)
		}
		if registry == nil || registry.RootDID == "" {
			return nil, fmt.Errorf("af server DID validation failed: registry or root DID is empty")
		}

		fmt.Printf("✅ Playground server DID created successfully: %s\n", registry.RootDID)

		// Backfill existing nodes with DIDs
		fmt.Println("🔄 Starting DID backfill for existing nodes...")
		ctx := context.Background()
		if err := didService.BackfillExistingNodes(ctx, storageProvider); err != nil {
			fmt.Printf("⚠️ DID backfill failed: %v\n", err)
		}

		fmt.Println("✅ DID and VC services initialized successfully!")
	} else {
		fmt.Println("⚠️ DID and VC services are DISABLED in configuration")
	}

	payloadStore := services.NewFilePayloadStore(dirs.PayloadsDir)

	webhookDispatcher := services.NewWebhookDispatcher(storageProvider, services.WebhookDispatcherConfig{
		Timeout:         cfg.Agents.ExecutionQueue.WebhookTimeout,
		MaxAttempts:     cfg.Agents.ExecutionQueue.WebhookMaxAttempts,
		RetryBackoff:    cfg.Agents.ExecutionQueue.WebhookRetryBackoff,
		MaxRetryBackoff: cfg.Agents.ExecutionQueue.WebhookMaxRetryBackoff,
	})
	if err := webhookDispatcher.Start(context.Background()); err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to start webhook dispatcher")
	}

	// Initialize observability forwarder for external webhook integration
	observabilityForwarder := services.NewObservabilityForwarder(storageProvider, services.ObservabilityForwarderConfig{
		BatchSize:       10,
		BatchTimeout:    time.Second,
		HTTPTimeout:     10 * time.Second,
		MaxAttempts:     3,
		RetryBackoff:    time.Second,
		MaxRetryBackoff: 30 * time.Second,
		WorkerCount:     2,
		QueueSize:       1000,
	})
	if err := observabilityForwarder.Start(context.Background()); err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to start observability forwarder")
	}

	// Initialize execution cleanup service
	cleanupService := handlers.NewExecutionCleanupService(storageProvider, cfg.Agents.ExecutionCleanup)

	adminPort := cfg.Agents.Port + 100
	envPort := os.Getenv("PLAYGROUND_ADMIN_GRPC_PORT")
	if envPort == "" {
		envPort = os.Getenv("AGENTS_ADMIN_GRPC_PORT") // Legacy fallback
	}
	if envPort != "" {
		if parsedPort, parseErr := strconv.Atoi(envPort); parseErr == nil {
			adminPort = parsedPort
		} else {
			logger.Logger.Warn().Err(parseErr).Str("value", envPort).Msg("invalid PLAYGROUND_ADMIN_GRPC_PORT, using default offset")
		}
	}

	// Initialize cloud provisioner if K8s cloud mode is enabled
	var cloudProvisioner *cloud.Provisioner
	if cfg.Cloud.Enabled && cfg.Cloud.Kubernetes.Enabled {
		logger.Logger.Info().
			Str("namespace", cfg.Cloud.Kubernetes.Namespace).
			Str("image", cfg.Cloud.Kubernetes.BotImage).
			Int("max_agents_per_org", cfg.Cloud.Kubernetes.MaxAgentsPerOrg).
			Msg("initializing cloud agent provisioner")

		k8sClient, err := cloud.NewInClusterClient()
		if err != nil {
			logger.Logger.Warn().Err(err).Msg("K8s in-cluster client unavailable — cloud provisioning disabled")
		} else {
			cloudProvisioner = cloud.NewProvisioner(cfg.Cloud, k8sClient)
			// Sync existing cloud agents from K8s
			go func() {
				ctx := context.Background()
				if err := cloudProvisioner.Sync(ctx); err != nil {
					logger.Logger.Warn().Err(err).Msg("failed to sync cloud nodes from K8s")
				}
			}()
		}
	}

	// Initialize Space store based on storage mode.
	// SpaceStore uses the same database connection as the main storage provider.
	var spaceStore spaces.Store
	if cfg.Storage.Mode == "postgres" {
		// For postgres mode, get the *sql.DB from the storage provider.
		// The postgres storage exposes DB() for this purpose.
		if pgStore, ok := storageProvider.(interface{ RawDB() interface{} }); ok {
			if db, ok := pgStore.RawDB().(*sql.DB); ok {
				spaceStore = spaces.NewPostgresStore(db)
			}
		}
		if spaceStore == nil {
			// Fallback: create a standalone connection using the postgres DSN.
			logger.Logger.Warn().Msg("Space store: falling back to standalone postgres connection")
		}
	}
	if spaceStore == nil {
		// Local mode: use SQLite space store with the same data directory.
		dbPath := filepath.Join(dirs.DataDir, "spaces.db")
		sqliteDB, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open spaces SQLite DB: %w", err)
		}
		sqliteStore := spaces.NewSQLiteStore(sqliteDB)
		if err := sqliteStore.Initialize(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to initialize spaces SQLite store: %w", err)
		}
		spaceStore = sqliteStore
	}

	return &PlaygroundServer{
		storage:               storageProvider,
		cache:                 cacheProvider,
		Router:                Router,
		uiService:             uiService,
		executionsUIService:   executionsUIService,
		healthMonitor:         healthMonitor,
		presenceManager:       presenceManager,
		statusManager:         statusManager,
		botService:          botService,
		nodeClient:           nodeClient,
		config:                cfg,
		keystoreService:       keystoreService,
		didService:            didService,
		vcService:             vcService,
		didRegistry:           didRegistry,
		playgroundHome:        playgroundHome,
		cleanupService:        cleanupService,
		payloadStore:          payloadStore,
		webhookDispatcher:        webhookDispatcher,
		observabilityForwarder:   observabilityForwarder,
		registryWatcherCancel:    nil,
		zapAdminPort:             adminPort,
		cloudProvisioner:         cloudProvisioner,
		spaceStore:               spaceStore,
	}, nil
}

// Start initializes and starts the PlaygroundServer.
func (s *PlaygroundServer) Start() error {
	// Setup routes
	s.setupRoutes()

	// Start status manager service in background
	go s.statusManager.Start()

	if s.presenceManager != nil {
		go s.presenceManager.Start()

		// Recover presence leases from database
		go func() {
			ctx := context.Background()
			if err := s.presenceManager.RecoverFromDatabase(ctx, s.storage); err != nil {
				logger.Logger.Error().Err(err).Msg("Failed to recover presence leases from database")
			}
		}()
	}

	// Start health monitor service in background
	go s.healthMonitor.Start()

	// Recover previously registered nodes and check their health
	go func() {
		ctx := context.Background()
		if err := s.healthMonitor.RecoverFromDatabase(ctx); err != nil {
			logger.Logger.Error().Err(err).Msg("Failed to recover nodes from database")
		}
	}()

	// Start execution cleanup service in background
	ctx := context.Background()
	if err := s.cleanupService.Start(ctx); err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to start execution cleanup service")
		// Don't fail server startup if cleanup service fails to start
	}

	// Start bot event heartbeat (30 second intervals)
	events.StartHeartbeat(30 * time.Second)

	// Start node event heartbeat (30 second intervals)
	events.StartNodeHeartbeat(30 * time.Second)

	if s.registryWatcherCancel == nil {
		cancel, err := StartPackageRegistryWatcher(context.Background(), s.playgroundHome, s.storage)
		if err != nil {
			logger.Logger.Error().Err(err).Msg("failed to start package registry watcher")
		} else {
			s.registryWatcherCancel = cancel
		}
	}

	// Start ZAP admin node for zero-copy admin operations
	zapAdmin, err := startZAPAdminNode(s.zapAdminPort, s.storage)
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to start ZAP admin node (continuing with REST only)")
	} else {
		s.zapAdmin = zapAdmin
	}

	// Register REST admin endpoints on the main router
	registerAdminRESTRoutes(s.Router, s.storage)

	// Start HTTP server
	return s.Router.Run(":" + strconv.Itoa(s.config.Agents.Port))
}

// Stop gracefully shuts down the PlaygroundServer.
func (s *PlaygroundServer) Stop() error {
	if s.zapAdmin != nil {
		s.zapAdmin.stop()
	}

	// Stop status manager service
	if s.statusManager != nil {
		s.statusManager.Stop()
	}

	if s.presenceManager != nil {
		s.presenceManager.Stop()
	}

	// Stop health monitor service
	s.healthMonitor.Stop()

	// Stop execution cleanup service
	if s.cleanupService != nil {
		if err := s.cleanupService.Stop(); err != nil {
			logger.Logger.Error().Err(err).Msg("Failed to stop execution cleanup service")
		}
	}

	if s.registryWatcherCancel != nil {
		s.registryWatcherCancel()
		s.registryWatcherCancel = nil
	}

	// Stop UI service heartbeat
	if s.uiService != nil {
		s.uiService.StopHeartbeat()
	}

	// Stop observability forwarder
	if s.observabilityForwarder != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.observabilityForwarder.Stop(ctx); err != nil {
			logger.Logger.Error().Err(err).Msg("Failed to stop observability forwarder")
		}
	}

	// HTTP server shutdown is handled by the caller (gin.Engine.Run blocks until killed).
	// WebSocket and gRPC connections are closed when the process exits.
	return nil
}

// unregisterAgentFromMonitoring removes an agent from health monitoring
func (s *PlaygroundServer) unregisterAgentFromMonitoring(c *gin.Context) {
	nodeID := c.Param("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id is required"})
		return
	}

	if s.healthMonitor != nil {
		s.healthMonitor.UnregisterAgent(nodeID)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("Agent %s unregistered from health monitoring", nodeID),
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "health monitor not available"})
	}
}

// healthCheckHandler provides comprehensive health check for container orchestration
func (s *PlaygroundServer) healthCheckHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	healthStatus := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   os.Getenv("PLAYGROUND_VERSION"),
		"checks":    gin.H{},
	}

	allHealthy := true
	checks := healthStatus["checks"].(gin.H)

	// Storage health check
	if s.storage != nil || s.storageHealthOverride != nil {
		storageHealth := s.checkStorageHealth(ctx)
		checks["storage"] = storageHealth
		if storageHealth["status"] != "healthy" {
			allHealthy = false
		}
	} else {
		checks["storage"] = gin.H{
			"status":  "unhealthy",
			"message": "storage not initialized",
		}
		allHealthy = false
	}

	// Cache health check
	if s.cache != nil || s.cacheHealthOverride != nil {
		cacheHealth := s.checkCacheHealth(ctx)
		checks["cache"] = cacheHealth
		if cacheHealth["status"] != "healthy" {
			allHealthy = false
		}
	} else {
		checks["cache"] = gin.H{
			"status":  "healthy",
			"message": "cache not configured (optional)",
		}
	}

	// Overall status
	if !allHealthy {
		healthStatus["status"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, healthStatus)
		return
	}

	c.JSON(http.StatusOK, healthStatus)
}

// checkStorageHealth performs storage-specific health checks
func (s *PlaygroundServer) checkStorageHealth(ctx context.Context) gin.H {
	if s.storageHealthOverride != nil {
		return s.storageHealthOverride(ctx)
	}

	startTime := time.Now()

	// For local storage, try a basic operation
	if err := ctx.Err(); err != nil {
		return gin.H{
			"status":  "unhealthy",
			"message": "context timeout during storage check",
		}
	}

	return gin.H{
		"status":        "healthy",
		"message":       "storage is responsive",
		"response_time": time.Since(startTime).Milliseconds(),
	}
}

// checkCacheHealth performs cache-specific health checks
func (s *PlaygroundServer) checkCacheHealth(ctx context.Context) gin.H {
	if s.cacheHealthOverride != nil {
		return s.cacheHealthOverride(ctx)
	}

	startTime := time.Now()

	// Try a simple cache operation
	testKey := "health_check_" + fmt.Sprintf("%d", time.Now().Unix())
	testValue := "ok"

	// Set a test value
	if err := s.cache.Set(testKey, testValue, time.Minute); err != nil {
		return gin.H{
			"status":        "unhealthy",
			"message":       fmt.Sprintf("cache set operation failed: %v", err),
			"response_time": time.Since(startTime).Milliseconds(),
		}
	}

	// Get the test value
	var retrieved string
	if err := s.cache.Get(testKey, &retrieved); err != nil {
		return gin.H{
			"status":        "unhealthy",
			"message":       fmt.Sprintf("cache get operation failed: %v", err),
			"response_time": time.Since(startTime).Milliseconds(),
		}
	}

	// Clean up
	if err := s.cache.Delete(testKey); err != nil {
		return gin.H{
			"status":        "unhealthy",
			"message":       fmt.Sprintf("cache delete operation failed: %v", err),
			"response_time": time.Since(startTime).Milliseconds(),
		}
	}

	return gin.H{
		"status":        "healthy",
		"message":       "cache is responsive",
		"response_time": time.Since(startTime).Milliseconds(),
	}
}

func (s *PlaygroundServer) setupRoutes() {
	// Configure CORS from configuration
	corsConfig := cors.Config{
		AllowOrigins:     s.config.API.CORS.AllowedOrigins,
		AllowMethods:     s.config.API.CORS.AllowedMethods,
		AllowHeaders:     s.config.API.CORS.AllowedHeaders,
		ExposeHeaders:    s.config.API.CORS.ExposedHeaders,
		AllowCredentials: s.config.API.CORS.AllowCredentials,
	}

	// Fallback to defaults if not configured
	if len(corsConfig.AllowOrigins) == 0 {
		corsConfig.AllowOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	}
	if len(corsConfig.AllowMethods) == 0 {
		corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if len(corsConfig.AllowHeaders) == 0 {
		corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key"}
	}

	s.Router.Use(cors.New(corsConfig))

	// Add request logging middleware
	s.Router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	// Add timeout middleware for all routes (1 hour for long-running executions)
	s.Router.Use(func(c *gin.Context) {
		// Set a timeout for the request
		ctx := c.Request.Context()
		timeoutCtx, cancel := context.WithTimeout(ctx, 3600*time.Second)
		defer cancel()

		c.Request = c.Request.WithContext(timeoutCtx)
		c.Next()
	})

	// OAuth proxy routes — registered BEFORE auth middleware since the user
	// doesn't have a token yet during login. These proxy token exchange and
	// userinfo requests to the IAM server, bypassing browser CORS restrictions.
	{
		iamEndpoint := s.config.IAM.Endpoint
		if iamEndpoint == "" {
			iamEndpoint = s.config.IAM.PublicEndpoint
		}
		authProxyCfg := handlers.AuthProxyConfig{
			TokenEndpoint:    iamEndpoint + "/oauth/token",
			UserinfoEndpoint: iamEndpoint + "/oauth/userinfo",
		}
		s.Router.POST("/auth/token", handlers.AuthTokenProxyHandler(authProxyCfg))
		s.Router.GET("/auth/userinfo", handlers.AuthUserinfoProxyHandler(authProxyCfg))
	}

	// IAM token authentication middleware (validates Bearer JWTs against hanzo.id)
	// Runs BEFORE API key auth — if IAM validates, request proceeds.
	// If no JWT present, falls through to API key auth.
	s.Router.Use(middleware.IAMAuth(middleware.IAMConfig{
		Enabled:        s.config.IAM.Enabled,
		Endpoint:       s.config.IAM.Endpoint,
		PublicEndpoint: s.config.IAM.PublicEndpoint,
		ClientID:       s.config.IAM.ClientID,
		ClientSecret:   s.config.IAM.ClientSecret,
		Organization:   s.config.IAM.Organization,
		Application:    s.config.IAM.Application,
		SkipPaths:      s.config.API.Auth.SkipPaths,
	}))
	if s.config.IAM.Enabled {
		logger.Logger.Info().
			Str("endpoint", s.config.IAM.Endpoint).
			Str("public_endpoint", s.config.IAM.PublicEndpoint).
			Str("client_id", s.config.IAM.ClientID).
			Str("org", s.config.IAM.Organization).
			Msg("🔐 IAM authentication enabled")
	}

	// API key authentication middleware (supports headers + api_key query param)
	s.Router.Use(middleware.APIKeyAuth(middleware.AuthConfig{
		APIKey:    s.config.API.Auth.APIKey,
		SkipPaths: s.config.API.Auth.SkipPaths,
	}))
	if s.config.API.Auth.APIKey != "" {
		logger.Logger.Info().Msg("🔐 API key authentication enabled")
	}

	// Expose Prometheus metrics
	s.Router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public health check endpoint for load balancers and container orchestration (e.g., Railway, K8s)
	s.Router.GET("/health", s.healthCheckHandler)

	// Serve UI files - embedded or filesystem based on availability
	if s.config.UI.Enabled {
		// Check if UI is embedded in the binary
		if s.config.UI.Mode == "embedded" && client.IsUIEmbedded() {
			// Use embedded UI
			client.RegisterUIRoutes(s.Router)
			fmt.Println("Using embedded UI files")
		} else {
			// Use filesystem UI
			distPath := s.resolveUIDistPath()

			// Serve static assets from dist
			s.Router.Static("/assets", filepath.Join(distPath, "assets"))
			s.Router.StaticFile("/favicon.svg", filepath.Join(distPath, "favicon.svg"))

			// Serve index.html at root (no-cache so deploys take effect immediately)
			s.Router.GET("/", func(c *gin.Context) {
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.File(filepath.Join(distPath, "index.html"))
			})

			// Redirect legacy /ui paths to root
			s.Router.GET("/ui", func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, "/")
			})
			s.Router.GET("/ui/*filepath", func(c *gin.Context) {
				path := c.Param("filepath")
				if path == "" || path == "/" {
					c.Redirect(http.StatusMovedPermanently, "/")
				} else {
					c.Redirect(http.StatusMovedPermanently, path)
				}
			})

			// SPA fallback
			s.Router.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") {
					c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
					return
				}
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.File(filepath.Join(distPath, "index.html"))
			})

			fmt.Printf("Using filesystem UI files from: %s\n", distPath)
		}
	}

	// All API routes under /api/v1
	agentAPI := s.Router.Group("/api/v1")
	{
		// Health check endpoint
		agentAPI.GET("/health", s.healthCheckHandler)

		// Discovery endpoints
		discovery := agentAPI.Group("/discovery")
		{
			discovery.GET("/capabilities", handlers.DiscoveryCapabilitiesHandler(s.storage))
		}

		// Agents management group
		agents := agentAPI.Group("/agents")
		{
			packagesHandler := ui.NewPackageHandler(s.storage)
			agents.GET("/packages", packagesHandler.ListPackagesHandler)
			agents.GET("/packages/:packageId/details", packagesHandler.GetPackageDetailsHandler)

			lifecycleHandler := ui.NewLifecycleHandler(s.storage, s.botService)
			agents.GET("/running", lifecycleHandler.ListRunningAgentsHandler)

			agents.GET("/:agentId/details", func(c *gin.Context) {
				agentID := c.Param("agentId")
				if agentID == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "agentId is required"})
					return
				}
				status, err := s.botService.GetBotStatus(agentID)
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
					return
				}
				c.JSON(http.StatusOK, status)
			})
			agents.GET("/:agentId/status", lifecycleHandler.GetBotStatusHandler)
			agents.POST("/:agentId/start", lifecycleHandler.StartAgentHandler)
			agents.POST("/:agentId/stop", lifecycleHandler.StopAgentHandler)
			agents.POST("/:agentId/reconcile", lifecycleHandler.ReconcileAgentHandler)

			configHandler := ui.NewConfigHandler(s.storage)
			agents.GET("/:agentId/config/schema", configHandler.GetConfigSchemaHandler)
			agents.GET("/:agentId/config", configHandler.GetConfigHandler)
			agents.POST("/:agentId/config", configHandler.SetConfigHandler)

			envHandler := ui.NewEnvHandler(s.storage, s.botService, s.playgroundHome)
			agents.GET("/:agentId/env", envHandler.GetEnvHandler)
			agents.PUT("/:agentId/env", envHandler.PutEnvHandler)
			agents.PATCH("/:agentId/env", envHandler.PatchEnvHandler)
			agents.DELETE("/:agentId/env/:key", envHandler.DeleteEnvVarHandler)

			agentExecutionHandler := ui.NewExecutionHandler(s.storage, s.payloadStore, s.webhookDispatcher)
			agents.GET("/:agentId/executions", agentExecutionHandler.ListExecutionsHandler)
			agents.GET("/:agentId/executions/:executionId", agentExecutionHandler.GetExecutionDetailsHandler)
		}

		// Node management endpoints
		agentAPI.POST("/nodes/register", handlers.RegisterNodeHandler(s.storage, s.uiService, s.didService, s.presenceManager))
		agentAPI.POST("/nodes", handlers.RegisterNodeHandler(s.storage, s.uiService, s.didService, s.presenceManager))
		agentAPI.POST("/nodes/register-serverless", handlers.RegisterServerlessAgentHandler(s.storage, s.uiService, s.didService, s.presenceManager))
		agentAPI.GET("/nodes", handlers.ListNodesHandler(s.storage))
		agentAPI.GET("/nodes/:node_id", handlers.GetNodeHandler(s.storage))
		agentAPI.POST("/nodes/:node_id/heartbeat", handlers.HeartbeatHandler(s.storage, s.uiService, s.healthMonitor, s.statusManager, s.presenceManager))
		agentAPI.DELETE("/nodes/:node_id/monitoring", s.unregisterAgentFromMonitoring)

		// Node status endpoints
		agentAPI.GET("/nodes/:node_id/status", handlers.GetNodeStatusHandler(s.statusManager))
		agentAPI.POST("/nodes/:node_id/status/refresh", handlers.RefreshNodeStatusHandler(s.statusManager))
		agentAPI.POST("/nodes/status/bulk", handlers.BulkNodeStatusHandler(s.statusManager, s.storage))
		agentAPI.POST("/nodes/status/refresh", handlers.RefreshAllNodeStatusHandler(s.statusManager, s.storage))

		// Node lifecycle endpoints
		agentAPI.POST("/nodes/:node_id/start", handlers.StartNodeHandler(s.statusManager, s.storage))
		agentAPI.POST("/nodes/:node_id/stop", handlers.StopNodeHandler(s.statusManager, s.storage))
		agentAPI.POST("/nodes/:node_id/lifecycle/status", handlers.UpdateLifecycleStatusHandler(s.storage, s.uiService, s.statusManager))
		agentAPI.PATCH("/nodes/:node_id/status", handlers.NodeStatusLeaseHandler(s.storage, s.statusManager, s.presenceManager, handlers.DefaultLeaseTTL))
		agentAPI.POST("/nodes/:node_id/actions/ack", handlers.NodeActionAckHandler(s.storage, s.presenceManager, handlers.DefaultLeaseTTL))
		agentAPI.POST("/nodes/:node_id/shutdown", handlers.NodeShutdownHandler(s.storage, s.statusManager, s.presenceManager))
		agentAPI.POST("/actions/claim", handlers.ClaimActionsHandler(s.storage, s.presenceManager, handlers.DefaultLeaseTTL))

		// Node summary and SSE events
		uiNodesHandler := ui.NewNodesHandler(s.uiService)
		agentAPI.GET("/nodes/summary", uiNodesHandler.GetNodesSummaryHandler)
		agentAPI.GET("/nodes/events", uiNodesHandler.StreamNodeEventsHandler)
		agentAPI.GET("/nodes/:node_id/details", uiNodesHandler.GetNodeDetailsHandler)

		// Node DID/VC endpoints
		nodeDIDHandler := ui.NewDIDHandler(s.storage, s.didService, s.vcService)
		agentAPI.GET("/nodes/:node_id/did", nodeDIDHandler.GetNodeDIDHandler)
		agentAPI.GET("/nodes/:node_id/vc-status", nodeDIDHandler.GetNodeVCStatusHandler)

		// Node MCP endpoints
		mcpHandler := ui.NewMCPHandler(s.uiService, s.nodeClient)
		agentAPI.GET("/nodes/:node_id/mcp/health", mcpHandler.GetMCPHealthHandler)
		agentAPI.GET("/nodes/:node_id/mcp/events", mcpHandler.GetMCPEventsHandler)
		agentAPI.GET("/nodes/:node_id/mcp/metrics", mcpHandler.GetMCPMetricsHandler)
		agentAPI.POST("/nodes/:node_id/mcp/servers/:alias/restart", mcpHandler.RestartMCPServerHandler)
		agentAPI.GET("/nodes/:node_id/mcp/servers/:alias/tools", mcpHandler.GetMCPToolsHandler)

		// Bot execution endpoints (legacy)
		agentAPI.POST("/bots/:botId", handlers.ExecuteBotHandler(s.storage))

		// Skill execution endpoints (legacy)
		agentAPI.POST("/skills/:skill_id", handlers.ExecuteSkillHandler(s.storage))

		// Unified execution endpoints (path-based)
		agentAPI.POST("/execute/:target", handlers.ExecuteHandler(s.storage, s.payloadStore, s.webhookDispatcher, s.config.Agents.ExecutionQueue.AgentCallTimeout))
		agentAPI.POST("/execute/async/:target", handlers.ExecuteAsyncHandler(s.storage, s.payloadStore, s.webhookDispatcher, s.config.Agents.ExecutionQueue.AgentCallTimeout))
		agentAPI.GET("/executions/:execution_id", handlers.GetExecutionStatusHandler(s.storage))
		agentAPI.POST("/executions/batch-status", handlers.BatchExecutionStatusHandler(s.storage))
		agentAPI.POST("/executions/:execution_id/status", handlers.UpdateExecutionStatusHandler(s.storage, s.payloadStore, s.webhookDispatcher, s.config.Agents.ExecutionQueue.AgentCallTimeout))

		// Execution notes
		agentAPI.POST("/executions/note", handlers.AddExecutionNoteHandler(s.storage))
		agentAPI.GET("/executions/:execution_id/notes", handlers.GetExecutionNotesHandler(s.storage))
		agentAPI.POST("/workflow/executions/events", handlers.WorkflowExecutionEventHandler(s.storage))

		// Execution UI endpoints (summary, stats, timeline, recent, details)
		uiExecutionsHandler := ui.NewExecutionHandler(s.storage, s.payloadStore, s.webhookDispatcher)
		agentAPI.GET("/executions/summary", uiExecutionsHandler.GetExecutionsSummaryHandler)
		agentAPI.GET("/executions/stats", uiExecutionsHandler.GetExecutionStatsHandler)
		agentAPI.GET("/executions/enhanced", uiExecutionsHandler.GetEnhancedExecutionsHandler)
		agentAPI.GET("/executions/events", uiExecutionsHandler.StreamExecutionEventsHandler)
		agentAPI.GET("/executions/:execution_id/details", uiExecutionsHandler.GetExecutionDetailsGlobalHandler)
		agentAPI.POST("/executions/:execution_id/webhook/retry", uiExecutionsHandler.RetryExecutionWebhookHandler)

		timelineHandler := ui.NewExecutionTimelineHandler(s.storage)
		agentAPI.GET("/executions/timeline", timelineHandler.GetExecutionTimelineHandler)

		recentActivityHandler := ui.NewRecentActivityHandler(s.storage)
		agentAPI.GET("/executions/recent", recentActivityHandler.GetRecentActivityHandler)

		// Execution DID/VC endpoints
		execDIDHandler := ui.NewDIDHandler(s.storage, s.didService, s.vcService)
		agentAPI.GET("/executions/:execution_id/vc", execDIDHandler.GetExecutionVCHandler)
		agentAPI.GET("/executions/:execution_id/vc-status", execDIDHandler.GetExecutionVCStatusHandler)
		agentAPI.POST("/executions/:execution_id/verify-vc", execDIDHandler.VerifyExecutionVCComprehensiveHandler)

		// Workflows management group
		workflows := agentAPI.Group("/workflows")
		{
			workflows.GET("/:workflowId/dag", handlers.GetWorkflowDAGHandler(s.storage))
			workflows.DELETE("/:workflowId/cleanup", handlers.CleanupWorkflowHandler(s.storage))
			wfDIDHandler := ui.NewDIDHandler(s.storage, s.didService, s.vcService)
			workflows.POST("/vc-status", wfDIDHandler.GetWorkflowVCStatusBatchHandler)
			workflows.GET("/:workflowId/vc-chain", wfDIDHandler.GetWorkflowVCChainHandler)
			workflows.POST("/:workflowId/verify-vc", wfDIDHandler.VerifyWorkflowVCComprehensiveHandler)

			workflowNotesHandler := ui.NewExecutionHandler(s.storage, s.payloadStore, s.webhookDispatcher)
			workflows.GET("/:workflowId/notes/events", workflowNotesHandler.StreamWorkflowNodeNotesHandler)
		}

		// Workflow runs (formerly /api/ui/v2)
		workflowRunsHandler := ui.NewWorkflowRunHandler(s.storage)
		agentAPI.GET("/workflow-runs", workflowRunsHandler.ListWorkflowRunsHandler)
		agentAPI.GET("/workflow-runs/:run_id", workflowRunsHandler.GetWorkflowRunDetailHandler)

		// Bots management group
		bots := agentAPI.Group("/bots")
		{
			botsHandler := ui.NewBotsHandler(s.storage)
			bots.GET("/all", botsHandler.GetAllBotsHandler)
			bots.GET("/events", botsHandler.StreamBotEventsHandler)
			bots.GET("/:botId/details", botsHandler.GetBotDetailsHandler)
			bots.GET("/:botId/metrics", botsHandler.GetPerformanceMetricsHandler)
			bots.GET("/:botId/executions", botsHandler.GetExecutionHistoryHandler)
			bots.GET("/:botId/templates", botsHandler.GetExecutionTemplatesHandler)
			bots.POST("/:botId/templates", botsHandler.SaveExecutionTemplateHandler)
		}

		// MCP system-wide endpoints
		agentAPI.GET("/mcp/status", mcpHandler.GetMCPStatusHandler)

		// Dashboard endpoints
		dashboard := agentAPI.Group("/dashboard")
		{
			dashboardHandler := ui.NewDashboardHandler(s.storage, s.botService)
			dashboard.GET("/summary", dashboardHandler.GetDashboardSummaryHandler)
			dashboard.GET("/enhanced", dashboardHandler.GetEnhancedDashboardSummaryHandler)
		}

		// Memory endpoints
		agentAPI.POST("/memory/set", handlers.SetMemoryHandler(s.storage))
		agentAPI.POST("/memory/get", handlers.GetMemoryHandler(s.storage))
		agentAPI.POST("/memory/delete", handlers.DeleteMemoryHandler(s.storage))
		agentAPI.GET("/memory/list", handlers.ListMemoryHandler(s.storage))

		// Vector Memory endpoints
		agentAPI.POST("/memory/vector", handlers.SetVectorHandler(s.storage))
		agentAPI.GET("/memory/vector/:key", handlers.GetVectorHandler(s.storage))
		agentAPI.POST("/memory/vector/search", handlers.SimilaritySearchHandler(s.storage))
		agentAPI.DELETE("/memory/vector/:key", handlers.DeleteVectorHandler(s.storage))
		agentAPI.POST("/memory/vector/set", handlers.SetVectorHandler(s.storage))
		agentAPI.POST("/memory/vector/delete", handlers.DeleteVectorHandler(s.storage))
		agentAPI.DELETE("/memory/vector/namespace", handlers.DeleteNamespaceVectorsHandler(s.storage))

		// Memory events endpoints
		memoryEventsHandler := handlers.NewMemoryEventsHandler(s.storage)
		agentAPI.GET("/memory/events/ws", memoryEventsHandler.WebSocketHandler)
		agentAPI.GET("/memory/events/sse", memoryEventsHandler.SSEHandler)
		agentAPI.GET("/memory/events/history", handlers.GetEventHistoryHandler(s.storage))

		// DID/VC service-backed endpoints
		logger.Logger.Debug().
			Bool("did_enabled", s.config.Features.DID.Enabled).
			Bool("did_service_available", s.didService != nil).
			Bool("vc_service_available", s.vcService != nil).
			Msg("DID Route Registration Check")

		if s.config.Features.DID.Enabled && s.didService != nil && s.vcService != nil {
			logger.Logger.Debug().Msg("Registering DID routes - all conditions met")
			didHandlers := handlers.NewDIDHandlers(s.didService, s.vcService)
			didHandlers.RegisterRoutes(agentAPI)

			agentAPI.GET("/did/agents-server", func(c *gin.Context) {
				agentsServerID, err := s.didService.GetPlaygroundServerID()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error":   "Failed to get playground server ID",
						"details": fmt.Sprintf("Playground server ID error: %v", err),
					})
					return
				}

				registry, err := s.didService.GetRegistry(agentsServerID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error":   "Failed to get af server DID",
						"details": fmt.Sprintf("Registry error: %v", err),
					})
					return
				}

				if registry == nil {
					c.JSON(http.StatusNotFound, gin.H{
						"error":   "Playground server DID not found",
						"details": "No DID registry exists for playground server 'default'. The DID system may not be properly initialized.",
					})
					return
				}

				if registry.RootDID == "" {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error":   "Playground server DID is empty",
						"details": "Registry exists but root DID is empty. The DID system may be corrupted.",
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"playground_server_id":  "default",
					"playground_server_did": registry.RootDID,
					"message":               "Playground server DID retrieved successfully",
				})
			})
		} else {
			logger.Logger.Warn().
				Bool("did_enabled", s.config.Features.DID.Enabled).
				Bool("did_service_available", s.didService != nil).
				Bool("vc_service_available", s.vcService != nil).
				Msg("DID routes NOT registered - conditions not met")

			// Register UI-only DID status/export routes when service routes aren't available
			didFallback := ui.NewDIDHandler(s.storage, s.didService, s.vcService)
			agentAPI.GET("/did/status", didFallback.GetDIDSystemStatusHandler)
			agentAPI.GET("/did/export/vcs", didFallback.ExportVCsHandler)
		}

		// DID/VC UI endpoints (non-conflicting with service routes, always registered)
		didUIHandler := ui.NewDIDHandler(s.storage, s.didService, s.vcService)
		agentAPI.GET("/did/:did/resolution-bundle", didUIHandler.GetDIDResolutionBundleHandler)
		agentAPI.GET("/did/:did/resolution-bundle/download", didUIHandler.DownloadDIDResolutionBundleHandler)
		agentAPI.GET("/vc/:vc_id/download", didUIHandler.DownloadVCHandler)
		agentAPI.POST("/vc/verify", didUIHandler.VerifyVCHandler)

		// Identity & Trust endpoints (DID Explorer and Credentials)
		identityHandler := ui.NewIdentityHandlers(s.storage)
		identityHandler.RegisterRoutes(agentAPI)

		// User preferences (notification sounds, voice settings)
		preferencesHandler := ui.NewPreferencesHandler(s.storage)
		agentAPI.GET("/preferences", preferencesHandler.GetPreferencesHandler)
		agentAPI.PUT("/preferences", preferencesHandler.PutPreferencesHandler)

		// Per-bot budget management
		budgetHandler := ui.NewBudgetHandler(s.storage)
		budgets := agentAPI.Group("/budgets")
		{
			budgets.GET("", budgetHandler.ListBudgets)
			budgets.GET("/:botId", budgetHandler.GetBudget)
			budgets.PUT("/:botId", budgetHandler.SetBudget)
			budgets.DELETE("/:botId", budgetHandler.DeleteBudget)
			budgets.GET("/:botId/check", budgetHandler.CheckBudget)
			budgets.GET("/:botId/spend", budgetHandler.GetSpendHistory)
		}

		// Cloud provisioning endpoints
		if s.cloudProvisioner != nil {
			s.registerCloudRoutes(agentAPI.Group("/cloud"))
			logger.Logger.Info().Msg("Cloud provisioning routes registered")
		}

		// Space API routes
		if s.spaceStore != nil {
			s.registerSpaceRoutes(agentAPI.Group("/spaces"))
			logger.Logger.Info().Msg("Space API routes registered")
		}

		// Settings API routes (observability webhook configuration)
		settings := agentAPI.Group("/settings")
		{
			obsHandler := ui.NewObservabilityWebhookHandler(s.storage, s.observabilityForwarder)
			settings.GET("/observability-webhook", obsHandler.GetWebhookHandler)
			settings.POST("/observability-webhook", obsHandler.SetWebhookHandler)
			settings.DELETE("/observability-webhook", obsHandler.DeleteWebhookHandler)
			settings.GET("/observability-webhook/status", obsHandler.GetStatusHandler)
			settings.POST("/observability-webhook/redrive", obsHandler.RedriveHandler)
			settings.GET("/observability-webhook/dlq", obsHandler.GetDeadLetterQueueHandler)
			settings.DELETE("/observability-webhook/dlq", obsHandler.ClearDeadLetterQueueHandler)
		}
	}

	// /v1/* → /api/v1/* rewrite for clean URLs via api.hanzo.bot/v1/
	s.Router.Any("/v1/*path", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1" + c.Param("path")
		s.Router.HandleContext(c)
	})

}

// registerCloudRoutes registers cloud provisioning endpoints on the given router group.
func (s *PlaygroundServer) registerCloudRoutes(cloudAPI *gin.RouterGroup) {
	cloudAPI.POST("/nodes/provision", handlers.CloudProvisionHandler(s.cloudProvisioner))
	cloudAPI.DELETE("/nodes/:node_id", handlers.CloudDeprovisionHandler(s.cloudProvisioner))
	cloudAPI.GET("/nodes", handlers.CloudListNodesHandler(s.cloudProvisioner))
	cloudAPI.GET("/nodes/:node_id", handlers.CloudGetNodeHandler(s.cloudProvisioner))
	cloudAPI.GET("/nodes/:node_id/logs", handlers.CloudGetLogsHandler(s.cloudProvisioner))
	cloudAPI.POST("/nodes/sync", handlers.CloudSyncHandler(s.cloudProvisioner))
	cloudAPI.POST("/teams/provision", handlers.TeamProvisionHandler(s.cloudProvisioner))
	cloudAPI.GET("/pricing", handlers.CloudPricingHandler())
	cloudAPI.GET("/presets", handlers.CloudPresetsHandler())
}

// registerSpaceRoutes registers the Space API endpoints on the given router group.
func (s *PlaygroundServer) registerSpaceRoutes(spacesAPI *gin.RouterGroup) {
	spaceHandler := handlers.NewSpaceHandler(s.spaceStore)
	spacesAPI.POST("", spaceHandler.CreateSpace)
	spacesAPI.GET("", spaceHandler.ListSpaces)
	spacesAPI.GET("/:id", spaceHandler.GetSpace)
	spacesAPI.PUT("/:id", spaceHandler.UpdateSpace)
	spacesAPI.DELETE("/:id", spaceHandler.DeleteSpace)

	spacesAPI.POST("/:id/members", spaceHandler.AddMember)
	spacesAPI.GET("/:id/members", spaceHandler.ListMembers)
	spacesAPI.DELETE("/:id/members/:uid", spaceHandler.RemoveMember)

	spaceNodeHandler := handlers.NewSpaceNodeHandler(s.spaceStore)
	spacesAPI.POST("/:id/nodes/register", spaceNodeHandler.RegisterNode)
	spacesAPI.GET("/:id/nodes", spaceNodeHandler.ListNodes)
	spacesAPI.DELETE("/:id/nodes/:nid", spaceNodeHandler.RemoveNode)
	spacesAPI.POST("/:id/nodes/:nid/heartbeat", spaceNodeHandler.Heartbeat)

	spaceBotHandler := handlers.NewSpaceBotHandler(s.spaceStore)
	spacesAPI.POST("/:id/bots", spaceBotHandler.CreateBot)
	spacesAPI.GET("/:id/bots", spaceBotHandler.ListBots)
	spacesAPI.DELETE("/:id/bots/:bid", spaceBotHandler.RemoveBot)
	spacesAPI.POST("/:id/bots/:bid/chat", spaceBotHandler.ChatMessage)
	spacesAPI.GET("/:id/bots/:bid/chat", spaceBotHandler.ChatHistory)

	nodeProxy := proxy.NewNodeProxy(s.spaceStore)
	spacesAPI.Any("/:id/nodes/:nid/v2/*path", nodeProxy.ProxyToNodeV2)
}

// resolveUIDistPath returns the filesystem path to the UI dist directory.
func (s *PlaygroundServer) resolveUIDistPath() string {
	distPath := s.config.UI.DistPath
	if distPath != "" {
		return distPath
	}

	execPath, err := os.Executable()
	if err != nil {
		distPath = filepath.Join("apps", "platform", "agents", "web", "client", "dist")
		if _, statErr := os.Stat(distPath); os.IsNotExist(statErr) {
			distPath = filepath.Join("web", "client", "dist")
		}
		return distPath
	}

	execDir := filepath.Dir(execPath)
	distPath = filepath.Join(execDir, "web", "client", "dist")

	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		distPath = filepath.Join(filepath.Dir(execDir), "apps", "platform", "agents", "web", "client", "dist")
	}

	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		altPath := filepath.Join("apps", "platform", "agents", "web", "client", "dist")
		if _, altErr := os.Stat(altPath); altErr == nil {
			distPath = altPath
		} else {
			distPath = filepath.Join("web", "client", "dist")
		}
	}

	return distPath
}

// generatePlaygroundServerID creates a deterministic af server ID based on the agents home directory.
// This ensures each agents instance has a unique ID while being deterministic for the same installation.
func generatePlaygroundServerID(playgroundHome string) string {
	// Use the absolute path of agents home to generate a deterministic ID
	absPath, err := filepath.Abs(playgroundHome)
	if err != nil {
		// Fallback to the original path if absolute path fails
		absPath = playgroundHome
	}

	// Create a hash of the agents home path to generate a unique but deterministic ID
	hash := sha256.Sum256([]byte(absPath))

	// Use first 16 characters of the hex hash as the af server ID
	// This provides uniqueness while keeping the ID manageable
	agentsServerID := hex.EncodeToString(hash[:])[:16]

	return agentsServerID
}
