package policy

import "time"

// DefaultSpacePolicy returns a sensible managed policy for a new space.
// Bots can read most resources and write to files/git, but need approval
// for deployments, billing, secrets, IAM, and DNS modifications.
func DefaultSpacePolicy(spaceID string) SpacePolicy {
	now := time.Now()
	return SpacePolicy{
		SpaceID:      spaceID,
		ApprovalMode: ApprovalManaged,
		DefaultRules: []PolicyRule{
			// Bots can read everything by default.
			{Resource: ResourceFiles, Permission: PermRead},
			{Resource: ResourceGit, Permission: PermRead},
			{Resource: ResourceCloud, Permission: PermRead},
			{Resource: ResourceKMS, Permission: PermRead},
			{Resource: ResourceNetwork, Permission: PermRead},
			// Bots can write to files and git in their space.
			{Resource: ResourceFiles, Permission: PermWrite},
			{Resource: ResourceGit, Permission: PermWrite},
			// Bots can execute shells.
			{Resource: ResourceShell, Permission: PermExecute},
			// Everything else requires approval.
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// BypassPolicy returns a full-access policy (bots go wild).
func BypassPolicy(spaceID string) SpacePolicy {
	now := time.Now()
	return SpacePolicy{
		SpaceID:      spaceID,
		ApprovalMode: ApprovalBypass,
		DefaultRules: []PolicyRule{
			{Resource: ResourceDNS, Permission: PermAdmin},
			{Resource: ResourceKMS, Permission: PermAdmin},
			{Resource: ResourceCloud, Permission: PermAdmin},
			{Resource: ResourceIAM, Permission: PermAdmin},
			{Resource: ResourceGit, Permission: PermAdmin},
			{Resource: ResourceNetwork, Permission: PermAdmin},
			{Resource: ResourceShell, Permission: PermAdmin},
			{Resource: ResourceFiles, Permission: PermAdmin},
			{Resource: ResourceSecrets, Permission: PermAdmin},
			{Resource: ResourceDeploy, Permission: PermAdmin},
			{Resource: ResourceBilling, Permission: PermAdmin},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TrustedPolicy returns a policy that auto-approves known-safe actions
// but still requires manual approval for sensitive operations like
// deployments, billing, and secrets management.
func TrustedPolicy(spaceID string) SpacePolicy {
	now := time.Now()
	return SpacePolicy{
		SpaceID:      spaceID,
		ApprovalMode: ApprovalTrusted,
		DefaultRules: []PolicyRule{
			{Resource: ResourceFiles, Permission: PermAdmin},
			{Resource: ResourceGit, Permission: PermAdmin},
			{Resource: ResourceCloud, Permission: PermWrite},
			{Resource: ResourceNetwork, Permission: PermWrite},
			{Resource: ResourceShell, Permission: PermExecute},
			{Resource: ResourceDNS, Permission: PermRead},
			{Resource: ResourceKMS, Permission: PermRead},
			{Resource: ResourceSecrets, Permission: PermRead},
			{Resource: ResourceDeploy, Permission: PermRead},
			{Resource: ResourceBilling, Permission: PermRead},
			{Resource: ResourceIAM, Permission: PermRead},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
