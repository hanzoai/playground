package services

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/pkg/types"
)

// DIDRegistry manages the storage and retrieval of DID registries using database-only operations.
type DIDRegistry struct {
	mu              sync.RWMutex
	registries      map[string]*types.DIDRegistry
	storageProvider storage.StorageProvider
}

// NewDIDRegistryWithStorage creates a new DID registry instance with database storage.
func NewDIDRegistryWithStorage(storageProvider storage.StorageProvider) *DIDRegistry {
	return &DIDRegistry{
		registries:      make(map[string]*types.DIDRegistry),
		storageProvider: storageProvider,
	}
}

// Initialize initializes the DID registry storage.
func (r *DIDRegistry) Initialize() error {
	if r.storageProvider == nil {
		return fmt.Errorf("storage provider not available")
	}

	// Load existing registries from database
	return r.loadRegistriesFromDatabase()
}

// GetRegistry retrieves a DID registry for a af server.
// Returns (nil, nil) if registry doesn't exist, (nil, error) for actual errors.
func (r *DIDRegistry) GetRegistry(agentsServerID string) (*types.DIDRegistry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registry, exists := r.registries[agentsServerID]
	if !exists {
		// Return nil, nil for "not found" to distinguish from actual errors
		return nil, nil
	}

	return registry, nil
}

// StoreRegistry stores a DID registry for a af server.
func (r *DIDRegistry) StoreRegistry(registry *types.DIDRegistry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store in memory
	r.registries[registry.PlaygroundServerID] = registry

	// Persist to database
	return r.saveRegistryToDatabase(registry)
}

// ListRegistries lists all af server registries.
func (r *DIDRegistry) ListRegistries() ([]*types.DIDRegistry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registries := make([]*types.DIDRegistry, 0, len(r.registries))
	for _, registry := range r.registries {
		registries = append(registries, registry)
	}

	return registries, nil
}

// DeleteRegistry deletes a DID registry for a af server.
func (r *DIDRegistry) DeleteRegistry(agentsServerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from memory
	delete(r.registries, agentsServerID)

	// TODO: Add database deletion method to storage interface
	// For now, we'll just remove from memory
	return nil
}

// UpdateBotStatus updates the status of an agent DID.
func (r *DIDRegistry) UpdateBotStatus(agentsServerID, nodeID string, status types.HanzoDIDStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	registry, exists := r.registries[agentsServerID]
	if !exists {
		return fmt.Errorf("registry not found for af server: %s", agentsServerID)
	}

	agentInfo, exists := registry.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("agent not found: %s", nodeID)
	}

	agentInfo.Status = status
	registry.Nodes[nodeID] = agentInfo

	// Persist changes to database
	return r.saveRegistryToDatabase(registry)
}

// FindDIDByComponent finds a DID by component type and function name.
func (r *DIDRegistry) FindDIDByComponent(agentsServerID, componentType, functionName string) (*types.DIDIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registry, exists := r.registries[agentsServerID]
	if !exists {
		return nil, fmt.Errorf("registry not found for af server: %s", agentsServerID)
	}

	// Search through all agent nodes
	for _, agentInfo := range registry.Nodes {
		switch componentType {
		case "agent":
			if agentInfo.NodeID == functionName {
				return &types.DIDIdentity{
					DID:            agentInfo.DID,
					PublicKeyJWK:   string(agentInfo.PublicKeyJWK),
					DerivationPath: agentInfo.DerivationPath,
					ComponentType:  "agent",
				}, nil
			}
		case "bot":
			for _, botInfo := range agentInfo.Bots {
				if botInfo.FunctionName == functionName {
					return &types.DIDIdentity{
						DID:            botInfo.DID,
						PublicKeyJWK:   string(botInfo.PublicKeyJWK),
						DerivationPath: botInfo.DerivationPath,
						ComponentType:  "bot",
						FunctionName:   botInfo.FunctionName,
					}, nil
				}
			}
		case "skill":
			for _, skillInfo := range agentInfo.Skills {
				if skillInfo.FunctionName == functionName {
					return &types.DIDIdentity{
						DID:            skillInfo.DID,
						PublicKeyJWK:   string(skillInfo.PublicKeyJWK),
						DerivationPath: skillInfo.DerivationPath,
						ComponentType:  "skill",
						FunctionName:   skillInfo.FunctionName,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("DID not found for component: %s/%s", componentType, functionName)
}

// GetNodeDIDs retrieves all DIDs for a specific agent node.
func (r *DIDRegistry) GetNodeDIDs(agentsServerID, nodeID string) (*types.DIDIdentityPackage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registry, exists := r.registries[agentsServerID]
	if !exists {
		return nil, fmt.Errorf("registry not found for af server: %s", agentsServerID)
	}

	agentInfo, exists := registry.Nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", nodeID)
	}

	// Build identity package (without private keys for security)
	botDIDs := make(map[string]types.DIDIdentity)
	for id, botInfo := range agentInfo.Bots {
		botDIDs[id] = types.DIDIdentity{
			DID:            botInfo.DID,
			PublicKeyJWK:   string(botInfo.PublicKeyJWK),
			DerivationPath: botInfo.DerivationPath,
			ComponentType:  "bot",
			FunctionName:   botInfo.FunctionName,
		}
	}

	skillDIDs := make(map[string]types.DIDIdentity)
	for id, skillInfo := range agentInfo.Skills {
		skillDIDs[id] = types.DIDIdentity{
			DID:            skillInfo.DID,
			PublicKeyJWK:   string(skillInfo.PublicKeyJWK),
			DerivationPath: skillInfo.DerivationPath,
			ComponentType:  "skill",
			FunctionName:   skillInfo.FunctionName,
		}
	}

	return &types.DIDIdentityPackage{
		NodeDID: types.DIDIdentity{
			DID:            agentInfo.DID,
			PublicKeyJWK:   string(agentInfo.PublicKeyJWK),
			DerivationPath: agentInfo.DerivationPath,
			ComponentType:  "agent",
		},
		BotDIDs:       botDIDs,
		SkillDIDs:          skillDIDs,
		PlaygroundServerID: agentsServerID,
	}, nil
}

// loadRegistriesFromDatabase loads all registries from the database.
func (r *DIDRegistry) loadRegistriesFromDatabase() error {
	if r.storageProvider == nil {
		return fmt.Errorf("storage provider not available")
	}

	ctx := context.Background()
	// Load af server DID information
	agentsServerDIDs, err := r.storageProvider.ListPlaygroundServerDIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list af server DIDs: %w", err)
	}

	// Create registries for each af server
	for _, agentsServerDIDInfo := range agentsServerDIDs {
		registry := &types.DIDRegistry{
			PlaygroundServerID: agentsServerDIDInfo.PlaygroundServerID,
			RootDID:            agentsServerDIDInfo.RootDID,
			MasterSeed:         agentsServerDIDInfo.MasterSeed,
			Nodes:         make(map[string]types.NodeDIDInfo),
			TotalDIDs:          0,
			CreatedAt:          agentsServerDIDInfo.CreatedAt,
			LastKeyRotation:    agentsServerDIDInfo.LastKeyRotation,
		}

		// Load agent DIDs for this af server
		agentDIDs, err := r.storageProvider.ListNodeDIDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to list agent DIDs: %w", err)
		}

		for _, agentDIDInfo := range agentDIDs {
			// Filter agents for this af server (assuming we can match by some criteria)
			// For now, we'll add all agents to the default af server
			// TODO: Add af server filtering when the storage interface supports it

			agentInfo := types.NodeDIDInfo{
				DID:                agentDIDInfo.DID,
				NodeID:        agentDIDInfo.NodeID,
				PlaygroundServerID: agentsServerDIDInfo.PlaygroundServerID,
				PublicKeyJWK:       agentDIDInfo.PublicKeyJWK,
				DerivationPath:     agentDIDInfo.DerivationPath,
				Status:             agentDIDInfo.Status,
				RegisteredAt:       agentDIDInfo.RegisteredAt,
				Bots:          make(map[string]types.BotDIDInfo),
				Skills:             make(map[string]types.SkillDIDInfo),
			}

			// Load component DIDs for this agent
			componentDIDs, err := r.storageProvider.ListComponentDIDs(ctx, agentDIDInfo.DID)
			if err != nil {
				return fmt.Errorf("failed to list component DIDs for agent %s: %w", agentDIDInfo.NodeID, err)
			}

			for _, componentDID := range componentDIDs {
				switch componentDID.ComponentType {
				case "bot":
					botInfo := types.BotDIDInfo{
						DID:            componentDID.ComponentDID,
						FunctionName:   componentDID.ComponentName,
						DerivationPath: fmt.Sprintf("m/44'/0'/0'/%d", componentDID.DerivationIndex),
						Capabilities:   []string{}, // TODO: Load from database
						ExposureLevel:  "private",  // TODO: Load from database
						CreatedAt:      componentDID.CreatedAt,
					}
					agentInfo.Bots[componentDID.ComponentName] = botInfo

				case "skill":
					skillInfo := types.SkillDIDInfo{
						DID:            componentDID.ComponentDID,
						FunctionName:   componentDID.ComponentName,
						DerivationPath: fmt.Sprintf("m/44'/0'/0'/%d", componentDID.DerivationIndex),
						Tags:           []string{}, // TODO: Load from database
						ExposureLevel:  "private",  // TODO: Load from database
						CreatedAt:      componentDID.CreatedAt,
					}
					agentInfo.Skills[componentDID.ComponentName] = skillInfo
				}
			}

			registry.Nodes[agentInfo.NodeID] = agentInfo
			registry.TotalDIDs++
		}

		r.registries[agentsServerDIDInfo.PlaygroundServerID] = registry
	}

	return nil
}

// saveRegistryToDatabase saves a registry to the database.
func (r *DIDRegistry) saveRegistryToDatabase(registry *types.DIDRegistry) error {
	if r.storageProvider == nil {
		return fmt.Errorf("storage provider not available")
	}

	ctx := context.Background()
	// Store af server DID information
	err := r.storageProvider.StorePlaygroundServerDID(
		ctx,
		registry.PlaygroundServerID,
		registry.RootDID,
		registry.MasterSeed,
		registry.CreatedAt,
		registry.LastKeyRotation,
	)
	if err != nil {
		return fmt.Errorf("failed to store af server DID: %w", err)
	}

	// Store each agent DID and its components using transaction-safe method
	for _, agentInfo := range registry.Nodes {
		// Extract derivation index from path (simplified)
		derivationIndex := 0 // TODO: Parse from agentInfo.DerivationPath

		// Prepare component DIDs for batch storage
		var components []storage.ComponentDIDRequest

		// Add bot DIDs
		for _, botInfo := range agentInfo.Bots {
			botDerivationIndex := 0 // TODO: Parse from botInfo.DerivationPath
			components = append(components, storage.ComponentDIDRequest{
				ComponentDID:    botInfo.DID,
				ComponentType:   "bot",
				ComponentName:   botInfo.FunctionName,
				PublicKeyJWK:    string(botInfo.PublicKeyJWK),
				DerivationIndex: botDerivationIndex,
			})
		}

		// Add skill DIDs
		for _, skillInfo := range agentInfo.Skills {
			skillDerivationIndex := 0 // TODO: Parse from skillInfo.DerivationPath
			components = append(components, storage.ComponentDIDRequest{
				ComponentDID:    skillInfo.DID,
				ComponentType:   "skill",
				ComponentName:   skillInfo.FunctionName,
				PublicKeyJWK:    string(skillInfo.PublicKeyJWK),
				DerivationIndex: skillDerivationIndex,
			})
		}

		// Use the enhanced storage method with transaction safety
		err := r.storageProvider.StoreNodeDIDWithComponents(
			ctx,
			agentInfo.NodeID,
			agentInfo.DID,
			registry.PlaygroundServerID, // Use af server ID instead of root DID
			string(agentInfo.PublicKeyJWK),
			derivationIndex,
			components,
		)
		if err != nil {
			// Enhanced error handling for different constraint types
			if validationErr, ok := err.(*storage.ValidationError); ok {
				return fmt.Errorf("validation failed for agent %s: %w", agentInfo.NodeID, validationErr)
			}
			if fkErr, ok := err.(*storage.ForeignKeyConstraintError); ok {
				return fmt.Errorf("foreign key constraint violation for agent %s: %w", agentInfo.NodeID, fkErr)
			}
			if dupErr, ok := err.(*storage.DuplicateDIDError); ok {
				log.Printf("Skipping duplicate DID entry during registry sync: %s (agent=%s)", dupErr.DID, agentInfo.NodeID)
				continue
			}
			return fmt.Errorf("failed to store agent DID %s with components: %w", agentInfo.NodeID, err)
		}
	}

	return nil
}
