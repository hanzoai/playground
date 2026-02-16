package services

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hanzoai/playground/control-plane/internal/config"
	"github.com/hanzoai/playground/control-plane/internal/storage"
	"github.com/hanzoai/playground/control-plane/pkg/types"

	"github.com/stretchr/testify/require"
)

func setupDIDTestEnvironment(t *testing.T) (*DIDService, *DIDRegistry, storage.StorageProvider, context.Context, string) {
	t.Helper()

	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: true, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}

	service := NewDIDService(cfg, ks, registry)

	agentsID := "agents-test"
	require.NoError(t, service.Initialize(agentsID))

	return service, registry, provider, ctx, agentsID
}

func TestDIDServiceRegisterNodeAndResolve(t *testing.T) {
	service, registry, provider, ctx, agentsID := setupDIDTestEnvironment(t)

	req := &types.DIDRegistrationRequest{
		NodeID: "agent-alpha",
		Bots:   []types.BotDefinition{{ID: "bot.fn"}},
		Skills:      []types.SkillDefinition{{ID: "skill.fn", Tags: []string{"analysis"}}},
	}

	resp, err := service.RegisterNode(req)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotEmpty(t, resp.IdentityPackage.NodeDID.DID)
	require.Contains(t, resp.IdentityPackage.BotDIDs, "bot.fn")
	require.Contains(t, resp.IdentityPackage.SkillDIDs, "skill.fn")

	storedRegistry, err := registry.GetRegistry(agentsID)
	require.NoError(t, err)
	require.NotNil(t, storedRegistry)
	require.Contains(t, storedRegistry.Nodes, "agent-alpha")

	agents, err := provider.ListNodeDIDs(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, agents)

	agentIdentity := resp.IdentityPackage.NodeDID
	resolved, err := service.ResolveDID(agentIdentity.DID)
	require.NoError(t, err)
	require.Equal(t, agentIdentity.DID, resolved.DID)

	botIdentity := resp.IdentityPackage.BotDIDs["bot.fn"]
	resolvedBot, err := service.ResolveDID(botIdentity.DID)
	require.NoError(t, err)
	require.Equal(t, botIdentity.DID, resolvedBot.DID)

	skillIdentity := resp.IdentityPackage.SkillDIDs["skill.fn"]
	resolvedSkill, err := service.ResolveDID(skillIdentity.DID)
	require.NoError(t, err)
	require.Equal(t, skillIdentity.DID, resolvedSkill.DID)
}

func TestDIDServiceValidateRegistryFailure(t *testing.T) {
	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: true, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}
	service := NewDIDService(cfg, ks, registry)

	err = service.validatePlaygroundServerRegistry()
	require.Error(t, err)

	agentsID := "agents-validate"
	require.NoError(t, service.Initialize(agentsID))
	require.NoError(t, service.validatePlaygroundServerRegistry())

	stored, err := registry.GetRegistry(agentsID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.False(t, stored.CreatedAt.IsZero())
	require.False(t, stored.LastKeyRotation.IsZero())
	_ = ctx
}

func TestDIDService_ResolveDID_RootDID(t *testing.T) {
	service, registry, _, _, agentsID := setupDIDTestEnvironment(t)

	// Get the root DID from registry
	storedRegistry, err := registry.GetRegistry(agentsID)
	require.NoError(t, err)
	require.NotNil(t, storedRegistry)
	require.NotEmpty(t, storedRegistry.RootDID)

	// Resolve root DID
	resolved, err := service.ResolveDID(storedRegistry.RootDID)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, storedRegistry.RootDID, resolved.DID)
	require.Equal(t, "playground_server", resolved.ComponentType)
	require.Equal(t, "m/44'/0'", resolved.DerivationPath)
	require.NotEmpty(t, resolved.PrivateKeyJWK)
	require.NotEmpty(t, resolved.PublicKeyJWK)
}

func TestDIDService_ResolveDID_NotFound(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	resolved, err := service.ResolveDID("did:key:invalid")
	require.Error(t, err)
	require.Nil(t, resolved)
	require.Contains(t, err.Error(), "DID not found")
}

func TestDIDService_ResolveDID_DisabledSystem(t *testing.T) {
	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: false, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}
	service := NewDIDService(cfg, ks, registry)

	resolved, err := service.ResolveDID("did:key:test")
	require.Error(t, err)
	require.Nil(t, resolved)
	require.Contains(t, err.Error(), "DID system is disabled")
	_ = ctx
}

func TestDIDService_RegisterNode_ExistingAgent_NoChanges(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent first time
	req1 := &types.DIDRegistrationRequest{
		NodeID: "agent-existing",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	resp1, err := service.RegisterNode(req1)
	require.NoError(t, err)
	require.True(t, resp1.Success)

	// Register same agent again with same components
	req2 := &types.DIDRegistrationRequest{
		NodeID: "agent-existing",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	resp2, err := service.RegisterNode(req2)
	require.NoError(t, err)
	require.True(t, resp2.Success)
	require.Contains(t, resp2.Message, "No changes detected")
	require.Equal(t, resp1.IdentityPackage.NodeDID.DID, resp2.IdentityPackage.NodeDID.DID)
}

func TestDIDService_PartialRegisterNode_NewComponents(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent with initial components
	req1 := &types.DIDRegistrationRequest{
		NodeID: "agent-partial",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	resp1, err := service.RegisterNode(req1)
	require.NoError(t, err)
	require.True(t, resp1.Success)

	// Partial registration with new components
	partialReq := &types.PartialDIDRegistrationRequest{
		NodeID:    "agent-partial",
		NewBotIDs: []string{"bot2"},
		NewSkillIDs:    []string{"skill2"},
		AllBots:   []types.BotDefinition{{ID: "bot1"}, {ID: "bot2"}},
		AllSkills:      []types.SkillDefinition{{ID: "skill1"}, {ID: "skill2"}},
	}

	resp2, err := service.PartialRegisterNode(partialReq)
	require.NoError(t, err)
	require.True(t, resp2.Success)
	require.Contains(t, resp2.Message, "Partial registration successful")
	require.Len(t, resp2.IdentityPackage.BotDIDs, 1) // Only new ones
	require.Len(t, resp2.IdentityPackage.SkillDIDs, 1)     // Only new ones
	require.Contains(t, resp2.IdentityPackage.BotDIDs, "bot2")
	require.Contains(t, resp2.IdentityPackage.SkillDIDs, "skill2")
}

func TestDIDService_PartialRegisterNode_DisabledSystem(t *testing.T) {
	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: false, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}
	service := NewDIDService(cfg, ks, registry)

	partialReq := &types.PartialDIDRegistrationRequest{
		NodeID:    "agent-test",
		NewBotIDs: []string{"bot1"},
		AllBots:   []types.BotDefinition{{ID: "bot1"}},
		AllSkills:      []types.SkillDefinition{},
	}

	resp, err := service.PartialRegisterNode(partialReq)
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.Error, "DID system is disabled")
	_ = ctx
}

func TestDIDService_DeregisterComponents_Success(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent with multiple components
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-deregister",
		Bots:   []types.BotDefinition{{ID: "bot1"}, {ID: "bot2"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}, {ID: "skill2"}},
	}

	resp, err := service.RegisterNode(req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Deregister some components
	deregReq := &types.ComponentDeregistrationRequest{
		NodeID:         "agent-deregister",
		BotIDsToRemove: []string{"bot1"},
		SkillIDsToRemove:    []string{"skill1"},
	}

	deregResp, err := service.DeregisterComponents(deregReq)
	require.NoError(t, err)
	require.True(t, deregResp.Success)
	require.Equal(t, 2, deregResp.RemovedCount)

	// Verify components were removed
	existingAgent, err := service.GetExistingNodeDID("agent-deregister")
	require.NoError(t, err)
	require.NotContains(t, existingAgent.Bots, "bot1")
	require.Contains(t, existingAgent.Bots, "bot2")
	require.NotContains(t, existingAgent.Skills, "skill1")
	require.Contains(t, existingAgent.Skills, "skill2")
}

func TestDIDService_DeregisterComponents_NotFound(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-deregister-notfound",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	_, err := service.RegisterNode(req)
	require.NoError(t, err)

	// Try to deregister non-existent components
	deregReq := &types.ComponentDeregistrationRequest{
		NodeID:         "agent-deregister-notfound",
		BotIDsToRemove: []string{"nonexistent-bot"},
		SkillIDsToRemove:    []string{"nonexistent-skill"},
	}

	deregResp, err := service.DeregisterComponents(deregReq)
	require.NoError(t, err)
	require.True(t, deregResp.Success)
	require.Equal(t, 0, deregResp.RemovedCount) // No components removed
}

func TestDIDService_DeregisterComponents_AgentNotFound(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	deregReq := &types.ComponentDeregistrationRequest{
		NodeID:         "nonexistent-agent",
		BotIDsToRemove: []string{"bot1"},
		SkillIDsToRemove:    []string{"skill1"},
	}

	deregResp, err := service.DeregisterComponents(deregReq)
	require.NoError(t, err)
	require.False(t, deregResp.Success)
	require.Contains(t, deregResp.Error, "agent not found")
}

func TestDIDService_PerformDifferentialAnalysis_NoChanges(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-diff",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	_, err := service.RegisterNode(req)
	require.NoError(t, err)

	// Perform differential analysis with same components
	result, err := service.PerformDifferentialAnalysis("agent-diff", []string{"bot1"}, []string{"skill1"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.RequiresUpdate)
	require.Empty(t, result.NewBotIDs)
	require.Empty(t, result.RemovedBotIDs)
	require.Empty(t, result.NewSkillIDs)
	require.Empty(t, result.RemovedSkillIDs)
	require.Len(t, result.UpdatedBotIDs, 1)
	require.Len(t, result.UpdatedSkillIDs, 1)
}

func TestDIDService_PerformDifferentialAnalysis_NewComponents(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-diff-new",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	_, err := service.RegisterNode(req)
	require.NoError(t, err)

	// Perform differential analysis with new components
	result, err := service.PerformDifferentialAnalysis("agent-diff-new", []string{"bot1", "bot2"}, []string{"skill1", "skill2"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.RequiresUpdate)
	require.Len(t, result.NewBotIDs, 1)
	require.Contains(t, result.NewBotIDs, "bot2")
	require.Len(t, result.NewSkillIDs, 1)
	require.Contains(t, result.NewSkillIDs, "skill2")
	require.Empty(t, result.RemovedBotIDs)
	require.Empty(t, result.RemovedSkillIDs)
}

func TestDIDService_PerformDifferentialAnalysis_RemovedComponents(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent with multiple components
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-diff-removed",
		Bots:   []types.BotDefinition{{ID: "bot1"}, {ID: "bot2"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}, {ID: "skill2"}},
	}

	_, err := service.RegisterNode(req)
	require.NoError(t, err)

	// Perform differential analysis with fewer components
	result, err := service.PerformDifferentialAnalysis("agent-diff-removed", []string{"bot1"}, []string{"skill1"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.RequiresUpdate)
	require.Empty(t, result.NewBotIDs)
	require.Empty(t, result.NewSkillIDs)
	require.Len(t, result.RemovedBotIDs, 1)
	require.Contains(t, result.RemovedBotIDs, "bot2")
	require.Len(t, result.RemovedSkillIDs, 1)
	require.Contains(t, result.RemovedSkillIDs, "skill2")
}

func TestDIDService_PerformDifferentialAnalysis_AgentNotFound(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	result, err := service.PerformDifferentialAnalysis("nonexistent-agent", []string{"bot1"}, []string{"skill1"})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to get existing agent")
}

func TestDIDService_GetExistingNodeDID_Success(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-get-existing",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	regResp, err := service.RegisterNode(req)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Get existing agent
	existingAgent, err := service.GetExistingNodeDID("agent-get-existing")
	require.NoError(t, err)
	require.NotNil(t, existingAgent)
	require.Equal(t, "agent-get-existing", existingAgent.NodeID)
	require.Equal(t, regResp.IdentityPackage.NodeDID.DID, existingAgent.DID)
	require.Len(t, existingAgent.Bots, 1)
	require.Len(t, existingAgent.Skills, 1)
}

func TestDIDService_GetExistingNodeDID_NotFound(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	existingAgent, err := service.GetExistingNodeDID("nonexistent-agent")
	require.Error(t, err)
	require.Nil(t, existingAgent)
	require.Contains(t, err.Error(), "agent not found")
}

func TestDIDService_GetExistingNodeDID_DisabledSystem(t *testing.T) {
	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: false, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}
	service := NewDIDService(cfg, ks, registry)

	existingAgent, err := service.GetExistingNodeDID("agent-test")
	require.Error(t, err)
	require.Nil(t, existingAgent)
	require.Contains(t, err.Error(), "DID system is disabled")
	_ = ctx
}

func TestDIDService_ListAllNodeDIDs_Success(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register multiple agents
	req1 := &types.DIDRegistrationRequest{
		NodeID: "agent-list-1",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{},
	}

	_, err := service.RegisterNode(req1)
	require.NoError(t, err)

	req2 := &types.DIDRegistrationRequest{
		NodeID: "agent-list-2",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{},
	}

	_, err = service.RegisterNode(req2)
	require.NoError(t, err)

	// List all agent DIDs
	agentDIDs, err := service.ListAllNodeDIDs()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(agentDIDs), 2)
}

func TestDIDService_ListAllNodeDIDs_DisabledSystem(t *testing.T) {
	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: false, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}
	service := NewDIDService(cfg, ks, registry)

	agentDIDs, err := service.ListAllNodeDIDs()
	require.Error(t, err)
	require.Nil(t, agentDIDs)
	require.Contains(t, err.Error(), "DID system is disabled")
	_ = ctx
}

func TestDIDService_RegisterNode_EmptyBotID(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent with empty bot ID (should be skipped)
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-empty-bot",
		Bots:   []types.BotDefinition{{ID: ""}, {ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: "skill1"}},
	}

	resp, err := service.RegisterNode(req)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Len(t, resp.IdentityPackage.BotDIDs, 1) // Only non-empty bot
	require.Contains(t, resp.IdentityPackage.BotDIDs, "bot1")
}

func TestDIDService_RegisterNode_EmptySkillID(t *testing.T) {
	service, _, _, _, _ := setupDIDTestEnvironment(t)

	// Register agent with empty skill ID (should be skipped)
	req := &types.DIDRegistrationRequest{
		NodeID: "agent-empty-skill",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{{ID: ""}, {ID: "skill1"}},
	}

	resp, err := service.RegisterNode(req)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Len(t, resp.IdentityPackage.SkillDIDs, 1) // Only non-empty skill
	require.Contains(t, resp.IdentityPackage.SkillDIDs, "skill1")
}

func TestDIDService_RegisterNode_DisabledSystem(t *testing.T) {
	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: false, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}
	service := NewDIDService(cfg, ks, registry)

	req := &types.DIDRegistrationRequest{
		NodeID: "agent-disabled",
		Bots:   []types.BotDefinition{{ID: "bot1"}},
		Skills:      []types.SkillDefinition{},
	}

	resp, err := service.RegisterNode(req)
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.Error, "DID system is disabled")
	_ = ctx
}

func TestDIDService_GetPlaygroundServerID(t *testing.T) {
	service, _, _, _, agentsID := setupDIDTestEnvironment(t)

	serverID, err := service.GetPlaygroundServerID()
	require.NoError(t, err)
	require.Equal(t, agentsID, serverID)
}

func TestDIDService_GetPlaygroundServerID_NotInitialized(t *testing.T) {
	provider, ctx := setupTestStorage(t)
	registry := NewDIDRegistryWithStorage(provider)
	require.NoError(t, registry.Initialize())

	keystoreDir := filepath.Join(t.TempDir(), "keys")
	ks, err := NewKeystoreService(&config.KeystoreConfig{Path: keystoreDir, Type: "local"})
	require.NoError(t, err)

	cfg := &config.DIDConfig{Enabled: true, Keystore: config.KeystoreConfig{Path: keystoreDir, Type: "local"}}
	service := NewDIDService(cfg, ks, registry)

	serverID, err := service.GetPlaygroundServerID()
	require.Error(t, err)
	require.Empty(t, serverID)
	require.Contains(t, err.Error(), "not initialized")
	_ = ctx
}
