package billing

import (
	"context"
	"fmt"
)

// ProvisionAllowance is the result of a billing pre-check.
type ProvisionAllowance struct {
	Allowed       bool   `json:"allowed"`
	Reason        string `json:"reason,omitempty"`
	BalanceCents  int    `json:"balance_cents"`
	RequiredCents int    `json:"required_cents"`
	HoursAfford   int    `json:"hours_afford"`
}

// CentsPerHour returns the hourly cost in cents for a compute tier slug.
// Falls back to 4 (pro tier default) for unknown slugs.
func CentsPerHour(slug string) int {
	tiers := map[string]int{
		"s-1vcpu-1gb":   1,
		"s-1vcpu-2gb":   2,
		"s-2vcpu-2gb":   3,
		"s-2vcpu-4gb":   4,
		"s-4vcpu-8gb":   7,
		"s-8vcpu-16gb":  14,
		"s-16vcpu-32gb": 29,
		"g-2vcpu-8gb":   7,
		"g-4vcpu-16gb":  14,
		"c-2vcpu-4gb":   6,
		"c-4vcpu-8gb":   13,
	}
	// Map preset IDs to their underlying slugs.
	presets := map[string]string{
		"starter": "s-1vcpu-2gb",
		"pro":     "s-2vcpu-4gb",
		"power":   "s-4vcpu-8gb",
		"gpu":     "g-2vcpu-8gb",
	}
	if mapped, ok := presets[slug]; ok {
		slug = mapped
	}
	if cents, ok := tiers[slug]; ok {
		return cents
	}
	return 4 // default to pro tier
}

// CentsPerHourVM returns the hourly cost in cents for VM instance types
// (AWS Mac, Windows, Linux VMs). Falls back to 4 for unknown types.
// Static lookup for billing gate checks — canonical pricing lives in billing.hanzo.ai.
// VM provisioning itself is handled by Visor (visor repo).
func CentsPerHourVM(instanceType string) int {
	vmTiers := map[string]int{
		"mac2.metal":         65,
		"mac2-m1ultra.metal": 500,
		"mac2-m2.metal":      65,
		"mac2-m2pro.metal":   65,
		"mac-m4.metal":       123,
		"mac-m4pro.metal":    123,
		"t3.medium":          4,
		"t3.large":           8,
		"s-1vcpu-2gb":        2,
		"s-2vcpu-4gb":        4,
	}
	if c, ok := vmTiers[instanceType]; ok {
		return c
	}
	return 4
}

// MinimumHours returns the prepaid minimum hours for an instance type.
// AWS Mac Dedicated Hosts require a 24-hour minimum due to Apple macOS
// licensing requirements — this is an Apple/macOS license restriction,
// not a Hanzo policy.
func MinimumHours(instanceType string) int {
	macTypes := map[string]bool{
		"mac2.metal":         true,
		"mac2-m1ultra.metal": true,
		"mac2-m2.metal":      true,
		"mac2-m2pro.metal":   true,
		"mac-m4.metal":       true,
		"mac-m4pro.metal":    true,
	}
	if macTypes[instanceType] {
		return 24
	}
	return 1
}

// CheckProvisionAllowance verifies the user has sufficient compute funds.
// minimumHours sets the prepaid minimum (1 for most instances, 24 for Mac).
// Everyone pays — add credit to launch.
func CheckProvisionAllowance(
	ctx context.Context,
	client *Client,
	userID string,
	token string,
	centsPerHour int,
	minimumHours int,
) (*ProvisionAllowance, error) {
	if minimumHours < 1 {
		minimumHours = 1
	}

	balance, err := client.GetBalance(ctx, userID, token)
	if err != nil {
		return nil, fmt.Errorf("billing gate: %w", err)
	}

	availableCents := int(balance.Available)
	requiredCents := centsPerHour * minimumHours
	hoursAfford := 0
	if centsPerHour > 0 {
		hoursAfford = availableCents / centsPerHour
	}

	if availableCents < requiredCents {
		hourLabel := "hour"
		if minimumHours > 1 {
			hourLabel = "hours"
		}
		reason := fmt.Sprintf(
			"Insufficient funds. You need at least $%.2f (%d %s at $%.2f/hr). Current balance: $%.2f. Add funds at https://billing.hanzo.ai",
			float64(requiredCents)/100.0,
			minimumHours,
			hourLabel,
			float64(centsPerHour)/100.0,
			float64(availableCents)/100.0,
		)
		if minimumHours == 24 {
			reason += " Note: macOS instances require a 24-hour minimum due to Apple licensing requirements."
		}
		return &ProvisionAllowance{
			Allowed:       false,
			Reason:        reason,
			BalanceCents:  availableCents,
			RequiredCents: requiredCents,
			HoursAfford:   hoursAfford,
		}, nil
	}

	return &ProvisionAllowance{
		Allowed:       true,
		BalanceCents:  availableCents,
		RequiredCents: requiredCents,
		HoursAfford:   hoursAfford,
	}, nil
}
