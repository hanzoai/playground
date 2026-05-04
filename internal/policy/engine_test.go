package policy

import (
	"context"
	"testing"
)

func TestDefaultPolicyAllowsReads(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(DefaultSpacePolicy("space-1"))

	resources := []ResourceType{ResourceFiles, ResourceGit, ResourceCloud, ResourceKMS, ResourceNetwork}
	for _, r := range resources {
		allowed, approval, _ := e.Check("bot-1", "space-1", r, PermRead)
		if !allowed {
			t.Errorf("expected read on %s to be allowed", r)
		}
		if approval {
			t.Errorf("expected read on %s to not require approval", r)
		}
	}
}

func TestDefaultPolicyAllowsFileAndGitWrite(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(DefaultSpacePolicy("space-1"))

	for _, r := range []ResourceType{ResourceFiles, ResourceGit} {
		allowed, approval, _ := e.Check("bot-1", "space-1", r, PermWrite)
		if !allowed {
			t.Errorf("expected write on %s to be allowed", r)
		}
		if approval {
			t.Errorf("expected write on %s to not require approval", r)
		}
	}
}

func TestDefaultPolicyAllowsShellExecute(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(DefaultSpacePolicy("space-1"))

	allowed, approval, _ := e.Check("bot-1", "space-1", ResourceShell, PermExecute)
	if !allowed {
		t.Errorf("expected shell execute to be allowed")
	}
	if approval {
		t.Errorf("expected shell execute to not require approval")
	}
}

func TestDefaultPolicyRequiresApprovalForDeploy(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(DefaultSpacePolicy("space-1"))

	allowed, approval, _ := e.Check("bot-1", "space-1", ResourceDeploy, PermWrite)
	if allowed {
		t.Errorf("expected deploy write to not be directly allowed")
	}
	if !approval {
		t.Errorf("expected deploy write to require approval")
	}
}

func TestDefaultPolicyRequiresApprovalForBilling(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(DefaultSpacePolicy("space-1"))

	allowed, approval, _ := e.Check("bot-1", "space-1", ResourceBilling, PermWrite)
	if allowed {
		t.Errorf("expected billing write to not be directly allowed")
	}
	if !approval {
		t.Errorf("expected billing write to require approval")
	}
}

func TestBypassModeAllowsEverything(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(BypassPolicy("space-1"))

	resources := []ResourceType{ResourceDNS, ResourceKMS, ResourceDeploy, ResourceBilling, ResourceSecrets, ResourceIAM}
	perms := []Permission{PermRead, PermWrite, PermExecute, PermAdmin}
	for _, r := range resources {
		for _, p := range perms {
			allowed, approval, _ := e.Check("bot-1", "space-1", r, p)
			if !allowed {
				t.Errorf("bypass: expected %s on %s to be allowed", p, r)
			}
			if approval {
				t.Errorf("bypass: expected %s on %s to not require approval", p, r)
			}
		}
	}
}

func TestBotPolicyOverridesSpace(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(DefaultSpacePolicy("space-1"))

	// Give this specific bot deploy write access.
	e.SetBotPolicy(BotPolicy{
		BotID:        "bot-deploy",
		SpaceID:      "space-1",
		ApprovalMode: ApprovalTrusted,
		Rules: []PolicyRule{
			{Resource: ResourceDeploy, Permission: PermWrite},
			{Resource: ResourceFiles, Permission: PermRead},
		},
	})

	// The bot-specific policy should grant deploy write.
	allowed, _, _ := e.Check("bot-deploy", "space-1", ResourceDeploy, PermWrite)
	if !allowed {
		t.Error("expected bot-specific policy to allow deploy write")
	}

	// Other bots still use space default.
	allowed, approval, _ := e.Check("bot-other", "space-1", ResourceDeploy, PermWrite)
	if allowed {
		t.Error("expected space default to deny deploy write for other bots")
	}
	if !approval {
		t.Error("expected space default to require approval for deploy write")
	}
}

func TestExplicitDeny(t *testing.T) {
	e := NewEngine()
	e.SetBotPolicy(BotPolicy{
		BotID:        "bot-1",
		SpaceID:      "space-1",
		ApprovalMode: ApprovalBypass,
		Rules: []PolicyRule{
			{Resource: ResourceBilling, Permission: PermDeny},
			{Resource: ResourceFiles, Permission: PermAdmin},
		},
	})

	allowed, approval, _ := e.Check("bot-1", "space-1", ResourceBilling, PermRead)
	if allowed {
		t.Error("expected explicit deny to override bypass")
	}
	if approval {
		t.Error("expected explicit deny to not suggest approval")
	}
}

func TestNoPolicyDenies(t *testing.T) {
	e := NewEngine()

	allowed, approval, reason := e.Check("bot-1", "space-1", ResourceFiles, PermRead)
	if allowed {
		t.Error("expected no policy to deny access")
	}
	if approval {
		t.Error("expected no policy to not require approval")
	}
	if reason != "no policy configured for bot or space" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestSetBypassMode(t *testing.T) {
	e := NewEngine()
	e.SetSpacePolicy(DefaultSpacePolicy("space-1"))

	if e.IsBypassMode("bot-1", "space-1") {
		t.Error("expected managed mode initially")
	}

	e.SetBypassMode("space-1", true)

	if !e.IsBypassMode("bot-1", "space-1") {
		t.Error("expected bypass mode after toggle")
	}

	e.SetBypassMode("space-1", false)

	if e.IsBypassMode("bot-1", "space-1") {
		t.Error("expected managed mode after toggle back")
	}
}

func TestSetBypassModeCreatesPolicy(t *testing.T) {
	e := NewEngine()
	// No space policy set yet.
	e.SetBypassMode("space-new", true)

	if !e.IsBypassMode("bot-1", "space-new") {
		t.Error("expected bypass mode on newly created space policy")
	}
}

func TestApprovalWorkflow(t *testing.T) {
	e := NewEngine()

	reqID := e.RequestApproval(ApprovalRequest{
		BotID:       "bot-1",
		SpaceID:     "space-1",
		Resource:    ResourceDeploy,
		Permission:  PermWrite,
		Description: "deploy to production",
	})

	if reqID == "" {
		t.Fatal("expected non-empty request ID")
	}

	// Should appear in pending.
	pending := e.PendingApprovals("space-1")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(pending))
	}
	if pending[0].ID != reqID {
		t.Error("pending approval ID mismatch")
	}

	// Approve it.
	if err := e.ResolveApproval(reqID, true, "admin@hanzo.ai"); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	// Should no longer be pending.
	pending = e.PendingApprovals("space-1")
	if len(pending) != 0 {
		t.Errorf("expected 0 pending approvals after resolve, got %d", len(pending))
	}
}

func TestResolveApprovalDeny(t *testing.T) {
	e := NewEngine()

	reqID := e.RequestApproval(ApprovalRequest{
		BotID:       "bot-1",
		SpaceID:     "space-1",
		Resource:    ResourceSecrets,
		Permission:  PermWrite,
		Description: "write secret",
	})

	if err := e.ResolveApproval(reqID, false, "admin@hanzo.ai"); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	pending := e.PendingApprovals("space-1")
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after denial, got %d", len(pending))
	}
}

func TestDoubleResolveErrors(t *testing.T) {
	e := NewEngine()

	reqID := e.RequestApproval(ApprovalRequest{
		BotID:       "bot-1",
		SpaceID:     "space-1",
		Resource:    ResourceDeploy,
		Permission:  PermWrite,
		Description: "deploy",
	})

	if err := e.ResolveApproval(reqID, true, "admin"); err != nil {
		t.Fatalf("first resolve failed: %v", err)
	}
	if err := e.ResolveApproval(reqID, true, "admin"); err == nil {
		t.Error("expected error on double resolve")
	}
}

func TestResolveNonexistent(t *testing.T) {
	e := NewEngine()
	if err := e.ResolveApproval("nonexistent-id", true, "admin"); err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestApprovalChannel(t *testing.T) {
	e := NewEngine()

	e.RequestApproval(ApprovalRequest{
		BotID:       "bot-1",
		SpaceID:     "space-1",
		Resource:    ResourceDeploy,
		Permission:  PermWrite,
		Description: "deploy",
	})

	select {
	case req := <-e.ApprovalRequests():
		if req.BotID != "bot-1" {
			t.Errorf("expected bot-1, got %s", req.BotID)
		}
	default:
		t.Error("expected approval notification on channel")
	}
}

func TestSensitiveActionApprovalInManaged(t *testing.T) {
	e := NewEngine()
	// Grant deploy write in a managed space.
	e.SetSpacePolicy(SpacePolicy{
		SpaceID:      "space-1",
		ApprovalMode: ApprovalManaged,
		DefaultRules: []PolicyRule{
			{Resource: ResourceDeploy, Permission: PermWrite},
		},
	})

	allowed, approval, _ := e.Check("bot-1", "space-1", ResourceDeploy, PermWrite)
	if !allowed {
		t.Error("expected deploy write to be allowed (rule exists)")
	}
	if !approval {
		t.Error("expected deploy write to still require approval in managed mode (sensitive)")
	}
}

func TestPermLevelHierarchy(t *testing.T) {
	// Admin grants read, write, and execute.
	e := NewEngine()
	e.SetBotPolicy(BotPolicy{
		BotID:        "bot-1",
		SpaceID:      "space-1",
		ApprovalMode: ApprovalTrusted,
		Rules: []PolicyRule{
			{Resource: ResourceFiles, Permission: PermAdmin},
		},
	})

	for _, p := range []Permission{PermRead, PermWrite, PermExecute, PermAdmin} {
		allowed, _, _ := e.Check("bot-1", "space-1", ResourceFiles, p)
		if !allowed {
			t.Errorf("expected admin rule to grant %s", p)
		}
	}
}

func TestDNSManagerBasicOperations(t *testing.T) {
	dm := NewDNSManager("https://dns.hanzo.ai", "test-token")
	ctx := context.Background()

	// Create a record.
	rec := DNSRecord{
		SpaceID: "space-1",
		Name:    "test.hanzo.bot",
		Type:    "A",
		Value:   "192.168.1.1",
	}
	if err := dm.CreateRecord(ctx, rec); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// List records.
	records, err := dm.ListRecords(ctx, "space-1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Name != "test.hanzo.bot" {
		t.Errorf("expected test.hanzo.bot, got %s", records[0].Name)
	}

	// Delete.
	if err := dm.DeleteRecord(ctx, records[0].ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	records, _ = dm.ListRecords(ctx, "space-1")
	if len(records) != 0 {
		t.Error("expected 0 records after delete")
	}
}

func TestDNSManagerValidation(t *testing.T) {
	dm := NewDNSManager("https://dns.hanzo.ai", "test-token")
	ctx := context.Background()

	// Missing name.
	err := dm.CreateRecord(ctx, DNSRecord{Type: "A", Value: "1.2.3.4"})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Invalid type.
	err = dm.CreateRecord(ctx, DNSRecord{Name: "test.hanzo.bot", Type: "MX", Value: "mail.hanzo.ai"})
	if err == nil {
		t.Error("expected error for invalid type")
	}

	// Missing value.
	err = dm.CreateRecord(ctx, DNSRecord{Name: "test.hanzo.bot", Type: "A"})
	if err == nil {
		t.Error("expected error for missing value")
	}

	// Delete nonexistent.
	err = dm.DeleteRecord(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for deleting nonexistent record")
	}
}

func TestEnsureBotDNS(t *testing.T) {
	dm := NewDNSManager("https://dns.hanzo.ai", "test-token")
	ctx := context.Background()

	fqdn, err := dm.EnsureBotDNS(ctx, "space-1", "bot-123", "My Cool Bot")
	if err != nil {
		t.Fatalf("ensure failed: %v", err)
	}
	if fqdn != "my-cool-bot.space-1.hanzo.bot" {
		t.Errorf("unexpected fqdn: %s", fqdn)
	}

	// Calling again should be idempotent.
	fqdn2, err := dm.EnsureBotDNS(ctx, "space-1", "bot-123", "My Cool Bot")
	if err != nil {
		t.Fatalf("ensure (idempotent) failed: %v", err)
	}
	if fqdn2 != fqdn {
		t.Errorf("expected same fqdn on second call, got %s", fqdn2)
	}

	// Should only have one record.
	records, _ := dm.ListRecords(ctx, "space-1")
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestSanitizeDNSLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Cool Bot", "my-cool-bot"},
		{"hello_world!", "hello-world"},
		{"---test---", "test"},
		{"UPPERCASE", "uppercase"},
		{"a", "a"},
		{"", ""},
	}

	for _, tt := range tests {
		result := sanitizeDNSLabel(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeDNSLabel(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultPolicies(t *testing.T) {
	dp := DefaultSpacePolicy("space-1")
	if dp.ApprovalMode != ApprovalManaged {
		t.Errorf("expected managed mode, got %s", dp.ApprovalMode)
	}
	if dp.SpaceID != "space-1" {
		t.Errorf("expected space-1, got %s", dp.SpaceID)
	}
	if len(dp.DefaultRules) == 0 {
		t.Error("expected non-empty default rules")
	}

	bp := BypassPolicy("space-2")
	if bp.ApprovalMode != ApprovalBypass {
		t.Errorf("expected bypass mode, got %s", bp.ApprovalMode)
	}
	if len(bp.DefaultRules) != 11 {
		t.Errorf("expected 11 bypass rules (all resources), got %d", len(bp.DefaultRules))
	}

	tp := TrustedPolicy("space-3")
	if tp.ApprovalMode != ApprovalTrusted {
		t.Errorf("expected trusted mode, got %s", tp.ApprovalMode)
	}
}

func TestGetBotPolicyReturnsNilForUnknown(t *testing.T) {
	e := NewEngine()
	bp := e.GetBotPolicy("unknown-bot", "unknown-space")
	if bp != nil {
		t.Error("expected nil for unknown bot and space")
	}
}

func TestPendingApprovalsFiltersBySpace(t *testing.T) {
	e := NewEngine()

	e.RequestApproval(ApprovalRequest{
		BotID:       "bot-1",
		SpaceID:     "space-1",
		Resource:    ResourceDeploy,
		Permission:  PermWrite,
		Description: "deploy",
	})
	// drain channel
	<-e.ApprovalRequests()

	e.RequestApproval(ApprovalRequest{
		BotID:       "bot-2",
		SpaceID:     "space-2",
		Resource:    ResourceDeploy,
		Permission:  PermWrite,
		Description: "deploy other",
	})
	<-e.ApprovalRequests()

	p1 := e.PendingApprovals("space-1")
	if len(p1) != 1 {
		t.Errorf("expected 1 pending for space-1, got %d", len(p1))
	}

	p2 := e.PendingApprovals("space-2")
	if len(p2) != 1 {
		t.Errorf("expected 1 pending for space-2, got %d", len(p2))
	}

	p3 := e.PendingApprovals("space-3")
	if len(p3) != 0 {
		t.Errorf("expected 0 pending for space-3, got %d", len(p3))
	}
}
