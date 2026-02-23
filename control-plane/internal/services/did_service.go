package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/config"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/pkg/types"
)

// DIDService handles DID generation, management, and resolution.
type DIDService struct {
	config             *config.DIDConfig
	keystore           *KeystoreService
	registry           *DIDRegistry
	agentsServerID string
}

// NewDIDService creates a new DID service instance.
func NewDIDService(cfg *config.DIDConfig, keystore *KeystoreService, registry *DIDRegistry) *DIDService {
	return &DIDService{
		config:             cfg,
		keystore:           keystore,
		registry:           registry,
		agentsServerID: "", // Will be set during initialization
	}
}

// Initialize initializes the DID service and creates af server master seed if needed.
func (s *DIDService) Initialize(agentsServerID string) error {
	if !s.config.Enabled {
		return nil
	}

	// Store the af server ID for dynamic resolution
	s.agentsServerID = agentsServerID

	// Check if af server already has a DID registry
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return fmt.Errorf("failed to check existing registry: %w", err)
	}

	if registry == nil {
		// Create new af server registry
		masterSeed := make([]byte, 32)
		if _, err := rand.Read(masterSeed); err != nil {
			return fmt.Errorf("failed to generate master seed: %w", err)
		}

		// Generate root DID from master seed
		rootDID, err := s.generateDIDFromSeed(masterSeed, "m/44'/0'")
		if err != nil {
			return fmt.Errorf("failed to generate root DID: %w", err)
		}

		// Create and store registry
		registry = &types.DIDRegistry{
			PlaygroundServerID: agentsServerID,
			MasterSeed:         masterSeed,
			RootDID:            rootDID,
			Nodes:         make(map[string]types.NodeDIDInfo),
			TotalDIDs:          1,
			CreatedAt:          time.Now(),
			LastKeyRotation:    time.Now(),
		}

		if err := s.registry.StoreRegistry(registry); err != nil {
			return fmt.Errorf("failed to store DID registry: %w", err)
		}

	}

	return nil
}

// GetPlaygroundServerID returns the af server ID for this DID service instance.
// This method provides dynamic af server ID resolution instead of hardcoded "default".
func (s *DIDService) GetPlaygroundServerID() (string, error) {
	if s.agentsServerID == "" {
		return "", fmt.Errorf("af server ID not initialized - call Initialize() first")
	}
	return s.agentsServerID, nil
}

// getPlaygroundServerID is an internal helper that returns the af server ID.
func (s *DIDService) getPlaygroundServerID() (string, error) {
	return s.GetPlaygroundServerID()
}

// validatePlaygroundServerRegistry ensures that the af server registry exists before operations.
func (s *DIDService) validatePlaygroundServerRegistry() error {
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return err
	}

	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return fmt.Errorf("failed to get af server registry: %w", err)
	}

	if registry == nil {
		return fmt.Errorf("af server registry not found for ID: %s - ensure Initialize() was called", agentsServerID)
	}

	return nil
}

// GetRegistry retrieves a DID registry for a af server.
func (s *DIDService) GetRegistry(agentsServerID string) (*types.DIDRegistry, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("DID system is disabled")
	}
	return s.registry.GetRegistry(agentsServerID)
}

// RegisterNode generates DIDs for an agent node and all its components.
// Enhanced to support partial registration for existing agents.
func (s *DIDService) RegisterNode(req *types.DIDRegistrationRequest) (*types.DIDRegistrationResponse, error) {
	if !s.config.Enabled {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   "DID system is disabled",
		}, nil
	}

	// Validate af server registry exists
	if err := s.validatePlaygroundServerRegistry(); err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("af server registry validation failed: %v", err),
		}, nil
	}

	// Check if agent already exists
	existingAgent, err := s.GetExistingNodeDID(req.NodeID)
	if err != nil && err.Error() != fmt.Sprintf("agent not found: %s", req.NodeID) {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to check existing agent: %v", err),
		}, nil
	}

	if existingAgent != nil {
		// Perform differential analysis
		newBotIDs := extractBotIDs(req.Bots)
		newSkillIDs := extractSkillIDs(req.Skills)

		diffResult, err := s.PerformDifferentialAnalysis(req.NodeID, newBotIDs, newSkillIDs)
		if err != nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("differential analysis failed: %v", err),
			}, nil
		}

		if !diffResult.RequiresUpdate {
			// No changes needed, return existing identity package
			identityPackage := s.buildExistingIdentityPackage(existingAgent)
			return &types.DIDRegistrationResponse{
				Success:         true,
				Message:         "No changes detected, registration skipped",
				IdentityPackage: identityPackage,
			}, nil
		}

		// Handle partial registration
		return s.handlePartialRegistration(req, diffResult)
	}

	// Handle new registration (existing logic)
	return s.handleNewRegistration(req)
}

// handleNewRegistration handles registration for new agents (original logic).
func (s *DIDService) handleNewRegistration(req *types.DIDRegistrationRequest) (*types.DIDRegistrationResponse, error) {
	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get af server ID: %v", err),
		}, nil
	}

	// Get af server registry using dynamic ID
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get DID registry: %v", err),
		}, nil
	}

	// Generate af server hash for derivation path
	agentsServerHash := s.hashPlaygroundServerID(registry.PlaygroundServerID)

	// Get next agent index
	agentIndex := len(registry.Nodes)

	// Generate agent DID
	agentPath := fmt.Sprintf("m/44'/%d'/%d'", agentsServerHash, agentIndex)
	agentDID, agentPrivKey, agentPubKey, err := s.generateDIDWithKeys(registry.MasterSeed, agentPath)
	if err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to generate agent DID: %v", err),
		}, nil
	}

	// Generate bot DIDs
	botDIDs := make(map[string]types.DIDIdentity)
	botInfos := make(map[string]types.BotDIDInfo)

	logger.Logger.Debug().Msgf("🔍 DEBUG: Calling did_manager.register_agent() with %d bots and %d skills", len(req.Bots), len(req.Skills))

	validBotIndex := 0
	for i, bot := range req.Bots {
		// Skip bots with empty IDs to prevent malformed DIDs
		if bot.ID == "" {
			logger.Logger.Warn().Msgf("⚠️ Skipping bot at index %d with empty ID", i)
			continue
		}

		botPath := fmt.Sprintf("m/44'/%d'/%d'/0'/%d'", agentsServerHash, agentIndex, validBotIndex)
		botDID, botPrivKey, botPubKey, err := s.generateDIDWithKeys(registry.MasterSeed, botPath)
		if err != nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to generate bot DID for %s: %v", bot.ID, err),
			}, nil
		}

		botDIDs[bot.ID] = types.DIDIdentity{
			DID:            botDID,
			PrivateKeyJWK:  botPrivKey,
			PublicKeyJWK:   botPubKey,
			DerivationPath: botPath,
			ComponentType:  "bot",
			FunctionName:   bot.ID,
		}

		botInfos[bot.ID] = types.BotDIDInfo{
			DID:            botDID,
			FunctionName:   bot.ID,
			PublicKeyJWK:   json.RawMessage(botPubKey),
			DerivationPath: botPath,
			Capabilities:   []string{},
			ExposureLevel:  "internal",
			CreatedAt:      time.Now(),
		}

		validBotIndex++
		logger.Logger.Debug().Msgf("🔍 Created DID for bot %s: %s", bot.ID, botDID)
	}

	logger.Logger.Debug().Msgf("🔍 Successfully created %d bot DIDs out of %d total bots", len(botDIDs), len(req.Bots))

	// Generate skill DIDs
	skillDIDs := make(map[string]types.DIDIdentity)
	skillInfos := make(map[string]types.SkillDIDInfo)

	validSkillIndex := 0
	for i, skill := range req.Skills {
		// Skip skills with empty IDs to prevent malformed DIDs
		if skill.ID == "" {
			logger.Logger.Warn().Msgf("⚠️ Skipping skill at index %d with empty ID", i)
			continue
		}

		skillPath := fmt.Sprintf("m/44'/%d'/%d'/1'/%d'", agentsServerHash, agentIndex, validSkillIndex)
		skillDID, skillPrivKey, skillPubKey, err := s.generateDIDWithKeys(registry.MasterSeed, skillPath)
		if err != nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to generate skill DID for %s: %v", skill.ID, err),
			}, nil
		}

		skillDIDs[skill.ID] = types.DIDIdentity{
			DID:            skillDID,
			PrivateKeyJWK:  skillPrivKey,
			PublicKeyJWK:   skillPubKey,
			DerivationPath: skillPath,
			ComponentType:  "skill",
			FunctionName:   skill.ID,
		}

		skillInfos[skill.ID] = types.SkillDIDInfo{
			DID:            skillDID,
			FunctionName:   skill.ID,
			PublicKeyJWK:   json.RawMessage(skillPubKey),
			DerivationPath: skillPath,
			Tags:           skill.Tags,
			ExposureLevel:  "internal",
			CreatedAt:      time.Now(),
		}

		validSkillIndex++
		logger.Logger.Debug().Msgf("🔍 Created DID for skill %s: %s", skill.ID, skillDID)
	}

	logger.Logger.Debug().Msgf("🔍 Successfully created %d skill DIDs out of %d total skills", len(skillDIDs), len(req.Skills))

	// Create agent DID info
	agentDIDInfo := types.NodeDIDInfo{
		DID:            agentDID,
		NodeID:    req.NodeID,
		PublicKeyJWK:   json.RawMessage(agentPubKey),
		DerivationPath: agentPath,
		Bots:      botInfos,
		Skills:         skillInfos,
		Status:         types.HanzoDIDStatusActive,
		RegisteredAt:   time.Now(),
	}

	// Update registry
	registry.Nodes[req.NodeID] = agentDIDInfo
	registry.TotalDIDs += 1 + len(req.Bots) + len(req.Skills)

	// Store updated registry
	if err := s.registry.StoreRegistry(registry); err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to store updated registry: %v", err),
		}, nil
	}

	// Create identity package
	identityPackage := types.DIDIdentityPackage{
		NodeDID: types.DIDIdentity{
			DID:            agentDID,
			PrivateKeyJWK:  agentPrivKey,
			PublicKeyJWK:   agentPubKey,
			DerivationPath: agentPath,
			ComponentType:  "agent",
		},
		BotDIDs:       botDIDs,
		SkillDIDs:          skillDIDs,
		PlaygroundServerID: registry.PlaygroundServerID,
	}

	// Debug log the response structure
	botDIDKeys := make([]string, 0, len(botDIDs))
	for key := range botDIDs {
		botDIDKeys = append(botDIDKeys, key)
	}
	logger.Logger.Debug().Msgf("🔍 DEBUG: DID registration response: {'bot_dids': %v, 'skill_dids': %d}", botDIDKeys, len(skillDIDs))

	return &types.DIDRegistrationResponse{
		Success:         true,
		IdentityPackage: identityPackage,
		Message:         fmt.Sprintf("Successfully registered agent %s with %d bots and %d skills", req.NodeID, len(botDIDs), len(skillDIDs)),
	}, nil
}

// ResolveDID resolves a DID to its public key and metadata.
func (s *DIDService) ResolveDID(did string) (*types.DIDIdentity, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("DID system is disabled")
	}

	// Validate af server registry exists
	if err := s.validatePlaygroundServerRegistry(); err != nil {
		return nil, fmt.Errorf("af server registry validation failed: %w", err)
	}

	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return nil, fmt.Errorf("failed to get af server ID: %w", err)
	}

	// Get af server registry using dynamic ID
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get DID registry: %w", err)
	}

	// Check if this is the af server root DID
	if registry.RootDID == did {
		// Regenerate private key for root DID using root derivation path
		privateKeyJWK, err := s.regeneratePrivateKeyJWK(registry.MasterSeed, "m/44'/0'")
		if err != nil {
			return nil, fmt.Errorf("failed to regenerate private key for root DID %s: %w", did, err)
		}

		// Generate public key JWK for consistency
		publicKeyJWK, err := s.regeneratePublicKeyJWK(registry.MasterSeed, "m/44'/0'")
		if err != nil {
			return nil, fmt.Errorf("failed to regenerate public key for root DID %s: %w", did, err)
		}

		return &types.DIDIdentity{
			DID:            registry.RootDID,
			PrivateKeyJWK:  privateKeyJWK,
			PublicKeyJWK:   publicKeyJWK,
			DerivationPath: "m/44'/0'",
			ComponentType:  "playground_server",
		}, nil
	}

	// Search through all agent nodes and their components
	for _, agentInfo := range registry.Nodes {
		if agentInfo.DID == did {
			// Regenerate private key from master seed and derivation path
			privateKeyJWK, err := s.regeneratePrivateKeyJWK(registry.MasterSeed, agentInfo.DerivationPath)
			if err != nil {
				return nil, fmt.Errorf("failed to regenerate private key for agent DID %s: %w", did, err)
			}

			return &types.DIDIdentity{
				DID:            agentInfo.DID,
				PrivateKeyJWK:  privateKeyJWK,
				PublicKeyJWK:   string(agentInfo.PublicKeyJWK),
				DerivationPath: agentInfo.DerivationPath,
				ComponentType:  "agent",
			}, nil
		}

		// Check bots
		for _, botInfo := range agentInfo.Bots {
			if botInfo.DID == did {
				// Regenerate private key from master seed and derivation path
				privateKeyJWK, err := s.regeneratePrivateKeyJWK(registry.MasterSeed, botInfo.DerivationPath)
				if err != nil {
					return nil, fmt.Errorf("failed to regenerate private key for bot DID %s: %w", did, err)
				}

				return &types.DIDIdentity{
					DID:            botInfo.DID,
					PrivateKeyJWK:  privateKeyJWK,
					PublicKeyJWK:   string(botInfo.PublicKeyJWK),
					DerivationPath: botInfo.DerivationPath,
					ComponentType:  "bot",
					FunctionName:   botInfo.FunctionName,
				}, nil
			}
		}

		// Check skills
		for _, skillInfo := range agentInfo.Skills {
			if skillInfo.DID == did {
				// Regenerate private key from master seed and derivation path
				privateKeyJWK, err := s.regeneratePrivateKeyJWK(registry.MasterSeed, skillInfo.DerivationPath)
				if err != nil {
					return nil, fmt.Errorf("failed to regenerate private key for skill DID %s: %w", did, err)
				}

				return &types.DIDIdentity{
					DID:            skillInfo.DID,
					PrivateKeyJWK:  privateKeyJWK,
					PublicKeyJWK:   string(skillInfo.PublicKeyJWK),
					DerivationPath: skillInfo.DerivationPath,
					ComponentType:  "skill",
					FunctionName:   skillInfo.FunctionName,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("DID not found: %s", did)
}

// generateDIDWithKeys generates a DID with private and public keys from master seed and derivation path.
func (s *DIDService) generateDIDWithKeys(masterSeed []byte, derivationPath string) (string, string, string, error) {
	// Derive private key using simplified BIP32-style derivation
	privateKey, err := s.derivePrivateKey(masterSeed, derivationPath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to derive private key: %w", err)
	}

	// Generate Ed25519 key pair
	publicKey := privateKey.Public().(ed25519.PublicKey)

	// Generate DID:key
	did := s.generateDIDKey(publicKey)

	// Convert keys to JWK format
	privateKeyJWK, err := s.ed25519PrivateKeyToJWK(privateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to convert private key to JWK: %w", err)
	}

	publicKeyJWK, err := s.ed25519PublicKeyToJWK(publicKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to convert public key to JWK: %w", err)
	}

	return did, privateKeyJWK, publicKeyJWK, nil
}

// generateDIDFromSeed generates a DID from master seed and derivation path.
func (s *DIDService) generateDIDFromSeed(masterSeed []byte, derivationPath string) (string, error) {
	privateKey, err := s.derivePrivateKey(masterSeed, derivationPath)
	if err != nil {
		return "", fmt.Errorf("failed to derive private key: %w", err)
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)
	return s.generateDIDKey(publicKey), nil
}

// derivePrivateKey derives a private key from master seed using simplified BIP32-style derivation.
func (s *DIDService) derivePrivateKey(masterSeed []byte, derivationPath string) (ed25519.PrivateKey, error) {
	// Simplified derivation: hash master seed with derivation path
	h := sha256.New()
	h.Write(masterSeed)
	h.Write([]byte(derivationPath))
	derivedSeed := h.Sum(nil)

	// Generate Ed25519 private key from derived seed
	privateKey := ed25519.NewKeyFromSeed(derivedSeed)
	return privateKey, nil
}

// generateDIDKey generates a DID:key from an Ed25519 public key.
func (s *DIDService) generateDIDKey(publicKey ed25519.PublicKey) string {
	// DID:key format: did:key:z + base58(multicodec + public key)
	// For Ed25519, multicodec prefix is 0xed01
	multicodecKey := append([]byte{0xed, 0x01}, publicKey...)

	// Use base64 encoding for simplicity (in production, use base58)
	encoded := base64.RawURLEncoding.EncodeToString(multicodecKey)
	return fmt.Sprintf("did:key:z%s", encoded)
}

// ed25519PrivateKeyToJWK converts an Ed25519 private key to JWK format.
func (s *DIDService) ed25519PrivateKeyToJWK(privateKey ed25519.PrivateKey) (string, error) {
	publicKey := privateKey.Public().(ed25519.PublicKey)

	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(publicKey),
		"d":   base64.RawURLEncoding.EncodeToString(privateKey.Seed()),
		"use": "sig",
		"alg": "EdDSA",
	}

	jwkBytes, err := json.Marshal(jwk)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWK: %w", err)
	}

	return string(jwkBytes), nil
}

// ed25519PublicKeyToJWK converts an Ed25519 public key to JWK format.
func (s *DIDService) ed25519PublicKeyToJWK(publicKey ed25519.PublicKey) (string, error) {
	jwk := map[string]interface{}{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(publicKey),
		"use": "sig",
		"alg": "EdDSA",
	}

	jwkBytes, err := json.Marshal(jwk)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWK: %w", err)
	}

	return string(jwkBytes), nil
}

// hashPlaygroundServerID creates a deterministic hash of af server ID for derivation paths.
func (s *DIDService) hashPlaygroundServerID(agentsServerID string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(agentsServerID))
	return h.Sum32() % (1 << 31) // Ensure it fits in BIP32 hardened derivation
}

// regeneratePrivateKeyJWK regenerates a private key JWK from master seed and derivation path.
func (s *DIDService) regeneratePrivateKeyJWK(masterSeed []byte, derivationPath string) (string, error) {
	// Derive private key using the same method as during generation
	privateKey, err := s.derivePrivateKey(masterSeed, derivationPath)
	if err != nil {
		return "", fmt.Errorf("failed to derive private key: %w", err)
	}

	// Convert to JWK format
	privateKeyJWK, err := s.ed25519PrivateKeyToJWK(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to convert private key to JWK: %w", err)
	}

	return privateKeyJWK, nil
}

// regeneratePublicKeyJWK regenerates a public key JWK from master seed and derivation path.
func (s *DIDService) regeneratePublicKeyJWK(masterSeed []byte, derivationPath string) (string, error) {
	// Derive private key using the same method as during generation
	privateKey, err := s.derivePrivateKey(masterSeed, derivationPath)
	if err != nil {
		return "", fmt.Errorf("failed to derive private key: %w", err)
	}

	// Get public key from private key
	publicKey := privateKey.Public().(ed25519.PublicKey)

	// Convert to JWK format
	publicKeyJWK, err := s.ed25519PublicKeyToJWK(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to convert public key to JWK: %w", err)
	}

	return publicKeyJWK, nil
}

// ListAllNodeDIDs returns all registered agent DIDs from the registry.
func (s *DIDService) ListAllNodeDIDs() ([]string, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("DID system is disabled")
	}

	// Validate af server registry exists
	if err := s.validatePlaygroundServerRegistry(); err != nil {
		return nil, fmt.Errorf("af server registry validation failed: %w", err)
	}

	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return nil, fmt.Errorf("failed to get af server ID: %w", err)
	}

	// Get af server registry using dynamic ID
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get DID registry: %w", err)
	}

	var agentDIDs []string
	for _, agentInfo := range registry.Nodes {
		agentDIDs = append(agentDIDs, agentInfo.DID)
	}

	return agentDIDs, nil
}

// BackfillExistingNodes registers existing nodes that don't have DIDs
func (s *DIDService) BackfillExistingNodes(ctx context.Context, storageProvider storage.StorageProvider) error {
	if !s.config.Enabled {
		logger.Logger.Debug().Msg("🔍 DID system disabled, skipping backfill")
		return nil
	}

	logger.Logger.Debug().Msg("🔍 Starting DID backfill for existing nodes...")

	// Get all registered nodes
	nodes, err := storageProvider.ListNodes(ctx, types.BotFilters{})
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(nodes) == 0 {
		logger.Logger.Debug().Msg("🔍 No existing nodes found, backfill complete")
		return nil
	}

	// Validate af server registry exists
	if err := s.validatePlaygroundServerRegistry(); err != nil {
		return fmt.Errorf("af server registry validation failed: %w", err)
	}

	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return fmt.Errorf("failed to get af server ID: %w", err)
	}

	// Get current DID registry using dynamic ID
	registry, err := s.GetRegistry(agentsServerID)
	if err != nil {
		return fmt.Errorf("failed to get DID registry: %w", err)
	}

	backfillCount := 0
	skippedCount := 0

	for _, node := range nodes {
		// Check if node already has DID
		if registry != nil {
			if _, exists := registry.Nodes[node.ID]; exists {
				logger.Logger.Debug().Msgf("🔍 Node %s already has DID, skipping", node.ID)
				skippedCount++
				continue // Already has DID
			}
		}

		// Register node with DID system
		didReq := &types.DIDRegistrationRequest{
			NodeID: node.ID,
			Bots:   node.Bots,
			Skills:      node.Skills,
		}

		didResponse, err := s.RegisterNode(didReq)
		if err != nil {
			logger.Logger.Warn().Err(err).Msgf("⚠️ Failed to backfill DID for node %s", node.ID)
		} else if !didResponse.Success {
			logger.Logger.Warn().Msgf("⚠️ DID backfill unsuccessful for node %s: %s", node.ID, didResponse.Error)
		} else {
			logger.Logger.Debug().Msgf("✅ Backfilled DID for node %s: %s", node.ID, didResponse.IdentityPackage.NodeDID.DID)
			backfillCount++
		}
	}

	logger.Logger.Debug().Msgf("🎉 DID backfill completed: %d nodes processed, %d new DIDs created, %d nodes already had DIDs",
		len(nodes), backfillCount, skippedCount)
	return nil
}

// GetExistingNodeDID retrieves existing DID information for an agent node.
func (s *DIDService) GetExistingNodeDID(nodeID string) (*types.NodeDIDInfo, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("DID system is disabled")
	}

	// Validate af server registry exists
	if err := s.validatePlaygroundServerRegistry(); err != nil {
		return nil, fmt.Errorf("af server registry validation failed: %w", err)
	}

	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return nil, fmt.Errorf("failed to get af server ID: %w", err)
	}

	// Get af server registry using dynamic ID
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get DID registry: %w", err)
	}

	agentInfo, exists := registry.Nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", nodeID)
	}

	return &agentInfo, nil
}

// PerformDifferentialAnalysis compares existing vs new bots/skills to determine what needs to be updated.
func (s *DIDService) PerformDifferentialAnalysis(nodeID string, newBotIDs, newSkillIDs []string) (*types.DifferentialAnalysisResult, error) {
	existingAgent, err := s.GetExistingNodeDID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing agent: %w", err)
	}

	// Extract existing IDs
	existingBotIDs := make([]string, 0, len(existingAgent.Bots))
	for id := range existingAgent.Bots {
		existingBotIDs = append(existingBotIDs, id)
	}

	existingSkillIDs := make([]string, 0, len(existingAgent.Skills))
	for id := range existingAgent.Skills {
		existingSkillIDs = append(existingSkillIDs, id)
	}

	// Perform set operations
	result := &types.DifferentialAnalysisResult{
		NewBotIDs:     setDifference(newBotIDs, existingBotIDs),
		RemovedBotIDs: setDifference(existingBotIDs, newBotIDs),
		UpdatedBotIDs: setIntersection(newBotIDs, existingBotIDs),
		NewSkillIDs:        setDifference(newSkillIDs, existingSkillIDs),
		RemovedSkillIDs:    setDifference(existingSkillIDs, newSkillIDs),
		UpdatedSkillIDs:    setIntersection(newSkillIDs, existingSkillIDs),
	}

	result.RequiresUpdate = len(result.NewBotIDs) > 0 ||
		len(result.RemovedBotIDs) > 0 ||
		len(result.NewSkillIDs) > 0 ||
		len(result.RemovedSkillIDs) > 0

	logger.Logger.Debug().Msgf("🔍 Differential analysis for agent %s: new_bots=%d, removed_bots=%d, new_skills=%d, removed_skills=%d, requires_update=%v",
		nodeID, len(result.NewBotIDs), len(result.RemovedBotIDs), len(result.NewSkillIDs), len(result.RemovedSkillIDs), result.RequiresUpdate)

	return result, nil
}

// setDifference returns elements in slice a that are not in slice b.
func setDifference(a, b []string) []string {
	bMap := make(map[string]bool)
	for _, item := range b {
		bMap[item] = true
	}

	var result []string
	for _, item := range a {
		if !bMap[item] {
			result = append(result, item)
		}
	}
	return result
}

// setIntersection returns elements that are in both slice a and slice b.
func setIntersection(a, b []string) []string {
	bMap := make(map[string]bool)
	for _, item := range b {
		bMap[item] = true
	}

	var result []string
	for _, item := range a {
		if bMap[item] {
			result = append(result, item)
		}
	}
	return result
}

// extractBotIDs extracts bot IDs from bot definitions.
func extractBotIDs(bots []types.BotDefinition) []string {
	ids := make([]string, 0, len(bots))
	for _, bot := range bots {
		if bot.ID != "" {
			ids = append(ids, bot.ID)
		}
	}
	return ids
}

// extractSkillIDs extracts skill IDs from skill definitions.
func extractSkillIDs(skills []types.SkillDefinition) []string {
	ids := make([]string, 0, len(skills))
	for _, skill := range skills {
		if skill.ID != "" {
			ids = append(ids, skill.ID)
		}
	}
	return ids
}

// findBotByID finds a bot definition by ID.
func (s *DIDService) findBotByID(bots []types.BotDefinition, id string) *types.BotDefinition {
	for _, bot := range bots {
		if bot.ID == id {
			return &bot
		}
	}
	return nil
}

// findSkillByID finds a skill definition by ID.
func (s *DIDService) findSkillByID(skills []types.SkillDefinition, id string) *types.SkillDefinition {
	for _, skill := range skills {
		if skill.ID == id {
			return &skill
		}
	}
	return nil
}

// generateBotPath generates a derivation path for a bot.
func (s *DIDService) generateBotPath(nodeID, botID string) string {
	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to get af server ID for bot path generation")
		return ""
	}

	// Get registry to find agent index
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to get registry for bot path generation")
		return ""
	}

	// Generate af server hash for derivation path
	agentsServerHash := s.hashPlaygroundServerID(registry.PlaygroundServerID)

	// Find agent index by matching node ID in the registry map
	agentIndex := 0
	for nid := range registry.Nodes {
		if nid == nodeID {
			break
		}
		agentIndex++
	}

	// Count existing bots to get next index
	existingAgent := registry.Nodes[nodeID]
	botIndex := len(existingAgent.Bots)

	return fmt.Sprintf("m/44'/%d'/%d'/0'/%d'", agentsServerHash, agentIndex, botIndex)
}

// generateSkillPath generates a derivation path for a skill.
func (s *DIDService) generateSkillPath(nodeID, skillID string) string {
	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to get af server ID for skill path generation")
		return ""
	}

	// Get registry to find agent index
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to get registry for skill path generation")
		return ""
	}

	// Generate af server hash for derivation path
	agentsServerHash := s.hashPlaygroundServerID(registry.PlaygroundServerID)

	// Find agent index by matching node ID in the registry map
	agentIndex := 0
	for nid := range registry.Nodes {
		if nid == nodeID {
			break
		}
		agentIndex++
	}

	// Count existing skills to get next index
	existingAgent := registry.Nodes[nodeID]
	skillIndex := len(existingAgent.Skills)

	return fmt.Sprintf("m/44'/%d'/%d'/1'/%d'", agentsServerHash, agentIndex, skillIndex)
}

// buildExistingIdentityPackage builds an identity package from existing agent DID info.
func (s *DIDService) buildExistingIdentityPackage(existingAgent *types.NodeDIDInfo) types.DIDIdentityPackage {
	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to get af server ID for identity package")
		agentsServerID = "unknown"
	}

	// Build bot DIDs map
	botDIDs := make(map[string]types.DIDIdentity)
	for id, botInfo := range existingAgent.Bots {
		botDIDs[id] = types.DIDIdentity{
			DID:            botInfo.DID,
			PrivateKeyJWK:  "", // Don't include private keys in existing package
			PublicKeyJWK:   string(botInfo.PublicKeyJWK),
			DerivationPath: botInfo.DerivationPath,
			ComponentType:  "bot",
			FunctionName:   botInfo.FunctionName,
		}
	}

	// Build skill DIDs map
	skillDIDs := make(map[string]types.DIDIdentity)
	for id, skillInfo := range existingAgent.Skills {
		skillDIDs[id] = types.DIDIdentity{
			DID:            skillInfo.DID,
			PrivateKeyJWK:  "", // Don't include private keys in existing package
			PublicKeyJWK:   string(skillInfo.PublicKeyJWK),
			DerivationPath: skillInfo.DerivationPath,
			ComponentType:  "skill",
			FunctionName:   skillInfo.FunctionName,
		}
	}

	return types.DIDIdentityPackage{
		NodeDID: types.DIDIdentity{
			DID:            existingAgent.DID,
			PrivateKeyJWK:  "", // Don't include private keys in existing package
			PublicKeyJWK:   string(existingAgent.PublicKeyJWK),
			DerivationPath: existingAgent.DerivationPath,
			ComponentType:  "agent",
		},
		BotDIDs:       botDIDs,
		SkillDIDs:          skillDIDs,
		PlaygroundServerID: agentsServerID,
	}
}

// handlePartialRegistration handles partial registration for existing agents.
func (s *DIDService) handlePartialRegistration(req *types.DIDRegistrationRequest, diffResult *types.DifferentialAnalysisResult) (*types.DIDRegistrationResponse, error) {
	// Handle deregistration of removed components first
	if len(diffResult.RemovedBotIDs) > 0 || len(diffResult.RemovedSkillIDs) > 0 {
		deregReq := &types.ComponentDeregistrationRequest{
			NodeID:         req.NodeID,
			BotIDsToRemove: diffResult.RemovedBotIDs,
			SkillIDsToRemove:    diffResult.RemovedSkillIDs,
		}

		deregResponse, err := s.DeregisterComponents(deregReq)
		if err != nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("component deregistration failed: %v", err),
			}, nil
		}

		if !deregResponse.Success {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("component deregistration failed: %s", deregResponse.Error),
			}, nil
		}

		logger.Logger.Debug().Msgf("✅ Deregistered %d components for agent %s", deregResponse.RemovedCount, req.NodeID)
	}

	// Handle partial registration of new components
	if len(diffResult.NewBotIDs) > 0 || len(diffResult.NewSkillIDs) > 0 {
		partialReq := &types.PartialDIDRegistrationRequest{
			NodeID:        req.NodeID,
			NewBotIDs:     diffResult.NewBotIDs,
			NewSkillIDs:        diffResult.NewSkillIDs,
			UpdatedBotIDs: diffResult.UpdatedBotIDs,
			UpdatedSkillIDs:    diffResult.UpdatedSkillIDs,
			AllBots:       req.Bots,
			AllSkills:          req.Skills,
		}

		return s.PartialRegisterNode(partialReq)
	}

	// If we reach here, only removals were needed
	existingAgent, err := s.GetExistingNodeDID(req.NodeID)
	if err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get updated agent info: %v", err),
		}, nil
	}

	identityPackage := s.buildExistingIdentityPackage(existingAgent)
	return &types.DIDRegistrationResponse{
		Success:         true,
		Message:         fmt.Sprintf("Registration updated: removed %d bots, %d skills", len(diffResult.RemovedBotIDs), len(diffResult.RemovedSkillIDs)),
		IdentityPackage: identityPackage,
	}, nil
}

// PartialRegisterNode registers only new components for an existing agent.
func (s *DIDService) PartialRegisterNode(req *types.PartialDIDRegistrationRequest) (*types.DIDRegistrationResponse, error) {
	if !s.config.Enabled {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   "DID system is disabled",
		}, nil
	}

	// Validate af server registry exists
	if err := s.validatePlaygroundServerRegistry(); err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("af server registry validation failed: %v", err),
		}, nil
	}

	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get af server ID: %v", err),
		}, nil
	}

	// Get af server registry using dynamic ID
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get DID registry: %v", err),
		}, nil
	}

	existingAgent, exists := registry.Nodes[req.NodeID]
	if !exists {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("agent %s not found", req.NodeID),
		}, nil
	}

	// Generate DIDs for new bots only
	newBotDIDs := make(map[string]types.DIDIdentity)
	newBotInfos := make(map[string]types.BotDIDInfo)

	for _, botID := range req.NewBotIDs {
		bot := s.findBotByID(req.AllBots, botID)
		if bot == nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("bot %s not found in request", botID),
			}, nil
		}

		// Generate DID for new bot
		botPath := s.generateBotPath(req.NodeID, botID)
		if botPath == "" {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to generate derivation path for bot %s", botID),
			}, nil
		}

		botDID, privKey, pubKey, err := s.generateDIDWithKeys(registry.MasterSeed, botPath)
		if err != nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to generate DID for bot %s: %v", botID, err),
			}, nil
		}

		newBotDIDs[botID] = types.DIDIdentity{
			DID:            botDID,
			PrivateKeyJWK:  privKey,
			PublicKeyJWK:   pubKey,
			DerivationPath: botPath,
			ComponentType:  "bot",
			FunctionName:   botID,
		}

		newBotInfos[botID] = types.BotDIDInfo{
			DID:            botDID,
			FunctionName:   botID,
			PublicKeyJWK:   json.RawMessage(pubKey),
			DerivationPath: botPath,
			Capabilities:   []string{}, // Default empty capabilities
			ExposureLevel:  "internal",
			CreatedAt:      time.Now(),
		}

		logger.Logger.Debug().Msgf("🔍 Generated new DID for bot %s: %s", botID, botDID)
	}

	// Generate DIDs for new skills
	newSkillDIDs := make(map[string]types.DIDIdentity)
	newSkillInfos := make(map[string]types.SkillDIDInfo)

	for _, skillID := range req.NewSkillIDs {
		skill := s.findSkillByID(req.AllSkills, skillID)
		if skill == nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("skill %s not found in request", skillID),
			}, nil
		}

		// Generate DID for new skill
		skillPath := s.generateSkillPath(req.NodeID, skillID)
		if skillPath == "" {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to generate derivation path for skill %s", skillID),
			}, nil
		}

		skillDID, privKey, pubKey, err := s.generateDIDWithKeys(registry.MasterSeed, skillPath)
		if err != nil {
			return &types.DIDRegistrationResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to generate DID for skill %s: %v", skillID, err),
			}, nil
		}

		newSkillDIDs[skillID] = types.DIDIdentity{
			DID:            skillDID,
			PrivateKeyJWK:  privKey,
			PublicKeyJWK:   pubKey,
			DerivationPath: skillPath,
			ComponentType:  "skill",
			FunctionName:   skillID,
		}

		newSkillInfos[skillID] = types.SkillDIDInfo{
			DID:            skillDID,
			FunctionName:   skillID,
			PublicKeyJWK:   json.RawMessage(pubKey),
			DerivationPath: skillPath,
			Tags:           skill.Tags,
			ExposureLevel:  "internal",
			CreatedAt:      time.Now(),
		}

		logger.Logger.Debug().Msgf("🔍 Generated new DID for skill %s: %s", skillID, skillDID)
	}

	// Update existing agent info with new components
	for id, info := range newBotInfos {
		existingAgent.Bots[id] = info
	}
	for id, info := range newSkillInfos {
		existingAgent.Skills[id] = info
	}

	// Update registry
	registry.Nodes[req.NodeID] = existingAgent
	registry.TotalDIDs += len(newBotDIDs) + len(newSkillDIDs)

	if err := s.registry.StoreRegistry(registry); err != nil {
		return &types.DIDRegistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to store updated registry: %v", err),
		}, nil
	}

	// Build response with only new DIDs
	identityPackage := types.DIDIdentityPackage{
		NodeDID: types.DIDIdentity{
			DID:            existingAgent.DID,
			PrivateKeyJWK:  "", // Don't regenerate existing agent key
			PublicKeyJWK:   string(existingAgent.PublicKeyJWK),
			DerivationPath: existingAgent.DerivationPath,
			ComponentType:  "agent",
		},
		BotDIDs:       newBotDIDs,
		SkillDIDs:          newSkillDIDs,
		PlaygroundServerID: registry.PlaygroundServerID,
	}

	logger.Logger.Debug().Msgf("✅ Partial registration successful for agent %s: %d new bots, %d new skills",
		req.NodeID, len(newBotDIDs), len(newSkillDIDs))

	return &types.DIDRegistrationResponse{
		Success:         true,
		IdentityPackage: identityPackage,
		Message:         fmt.Sprintf("Partial registration successful: %d new bots, %d new skills", len(newBotDIDs), len(newSkillDIDs)),
	}, nil
}

// DeregisterComponents removes specific components from an agent's DID registry.
func (s *DIDService) DeregisterComponents(req *types.ComponentDeregistrationRequest) (*types.ComponentDeregistrationResponse, error) {
	if !s.config.Enabled {
		return &types.ComponentDeregistrationResponse{
			Success: false,
			Error:   "DID system is disabled",
		}, nil
	}

	// Validate af server registry exists
	if err := s.validatePlaygroundServerRegistry(); err != nil {
		return &types.ComponentDeregistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("af server registry validation failed: %v", err),
		}, nil
	}

	// Get af server ID dynamically
	agentsServerID, err := s.getPlaygroundServerID()
	if err != nil {
		return &types.ComponentDeregistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get af server ID: %v", err),
		}, nil
	}

	// Get af server registry using dynamic ID
	registry, err := s.registry.GetRegistry(agentsServerID)
	if err != nil {
		return &types.ComponentDeregistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get DID registry: %v", err),
		}, nil
	}

	existingAgent, exists := registry.Nodes[req.NodeID]
	if !exists {
		return &types.ComponentDeregistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("agent %s not found", req.NodeID),
		}, nil
	}

	removedCount := 0

	// Remove bots
	for _, botID := range req.BotIDsToRemove {
		if _, exists := existingAgent.Bots[botID]; exists {
			delete(existingAgent.Bots, botID)
			removedCount++
			logger.Logger.Debug().Msgf("🗑️ Removed bot DID: %s from agent %s", botID, req.NodeID)
		} else {
			logger.Logger.Warn().Msgf("⚠️ Bot %s not found in agent %s, skipping removal", botID, req.NodeID)
		}
	}

	// Remove skills
	for _, skillID := range req.SkillIDsToRemove {
		if _, exists := existingAgent.Skills[skillID]; exists {
			delete(existingAgent.Skills, skillID)
			removedCount++
			logger.Logger.Debug().Msgf("🗑️ Removed skill DID: %s from agent %s", skillID, req.NodeID)
		} else {
			logger.Logger.Warn().Msgf("⚠️ Skill %s not found in agent %s, skipping removal", skillID, req.NodeID)
		}
	}

	// Update registry
	registry.Nodes[req.NodeID] = existingAgent
	registry.TotalDIDs -= removedCount

	if err := s.registry.StoreRegistry(registry); err != nil {
		return &types.ComponentDeregistrationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to store updated registry: %v", err),
		}, nil
	}

	logger.Logger.Debug().Msgf("✅ Component deregistration successful for agent %s: removed %d components",
		req.NodeID, removedCount)

	return &types.ComponentDeregistrationResponse{
		Success:      true,
		RemovedCount: removedCount,
		Message:      fmt.Sprintf("Successfully removed %d components", removedCount),
	}, nil
}
