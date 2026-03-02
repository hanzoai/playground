package billing

import (
	"context"
	"fmt"
	"math"
)

// ProvisionAllowance is the result of a billing pre-check.
type ProvisionAllowance struct {
	Allowed       bool   `json:"allowed"`
	Reason        string `json:"reason,omitempty"`
	BalanceCents  int    `json:"balance_cents"`
	RequiredCents int    `json:"required_cents"`
	HoursAfford   int    `json:"hours_afford"`
}

// superAdmins bypass all billing checks (same list as bot gateway).
var superAdmins = map[string]bool{
	"a@hanzo.ai":  true,
	"z@hanzo.ai":  true,
	"z@zeekay.io": true,
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

// CheckProvisionAllowance verifies the user has at least 1 hour of compute
// funds at the given hourly rate. Super admins always pass.
func CheckProvisionAllowance(
	ctx context.Context,
	client *Client,
	userEmail string,
	userID string,
	token string,
	centsPerHour int,
) (*ProvisionAllowance, error) {
	// Super admin bypass
	if superAdmins[userEmail] {
		return &ProvisionAllowance{
			Allowed:      true,
			BalanceCents: math.MaxInt32,
			HoursAfford:  math.MaxInt32,
		}, nil
	}

	balance, err := client.GetBalance(ctx, userID, token)
	if err != nil {
		return nil, fmt.Errorf("billing gate: %w", err)
	}

	availableCents := int(balance.Available)
	requiredCents := centsPerHour // 1 hour minimum
	hoursAfford := 0
	if centsPerHour > 0 {
		hoursAfford = availableCents / centsPerHour
	}

	if availableCents < requiredCents {
		return &ProvisionAllowance{
			Allowed: false,
			Reason: fmt.Sprintf(
				"Insufficient funds. You need at least $%.2f (1 hour at $%.2f/hr). Current balance: $%.2f. Add funds at https://billing.hanzo.ai",
				float64(requiredCents)/100.0,
				float64(centsPerHour)/100.0,
				float64(availableCents)/100.0,
			),
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
