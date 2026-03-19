package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// DNSRecord represents a DNS record managed by the playground.
type DNSRecord struct {
	ID      string `json:"id"`
	SpaceID string `json:"space_id"`
	Name    string `json:"name"`    // e.g. "mybot.hanzo.bot"
	Type    string `json:"type"`    // A, AAAA, CNAME, TXT
	Value   string `json:"value"`
	TTL     int    `json:"ttl"`
	Managed bool   `json:"managed"` // managed by playground vs manual
}

// validDNSTypes are the allowed DNS record types.
var validDNSTypes = map[string]bool{
	"A":     true,
	"AAAA":  true,
	"CNAME": true,
	"TXT":   true,
}

// DNSManager handles DNS record management for spaces.
type DNSManager struct {
	endpoint string // CoreDNS API or cloud DNS provider
	token    string
	// In-memory store for records. In production this would be backed
	// by the DNS provider API, but we keep a local index for lookups.
	records map[string]*DNSRecord // recordID -> record
}

// NewDNSManager creates a DNS manager for the given endpoint.
func NewDNSManager(endpoint, token string) *DNSManager {
	return &DNSManager{
		endpoint: endpoint,
		token:    token,
		records:  make(map[string]*DNSRecord),
	}
}

// ListRecords returns DNS records for a space.
func (d *DNSManager) ListRecords(_ context.Context, spaceID string) ([]DNSRecord, error) {
	var result []DNSRecord
	for _, r := range d.records {
		if r.SpaceID == spaceID {
			result = append(result, *r)
		}
	}
	return result, nil
}

// CreateRecord creates a DNS record (requires dns:write permission).
func (d *DNSManager) CreateRecord(_ context.Context, record DNSRecord) error {
	if record.Name == "" {
		return fmt.Errorf("dns record name is required")
	}
	if !validDNSTypes[record.Type] {
		return fmt.Errorf("invalid dns record type %q; must be A, AAAA, CNAME, or TXT", record.Type)
	}
	if record.Value == "" {
		return fmt.Errorf("dns record value is required")
	}
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.TTL <= 0 {
		record.TTL = 300 // 5 minute default
	}

	d.records[record.ID] = &record
	return nil
}

// DeleteRecord removes a DNS record.
func (d *DNSManager) DeleteRecord(_ context.Context, recordID string) error {
	if _, ok := d.records[recordID]; !ok {
		return fmt.Errorf("dns record %s not found", recordID)
	}
	delete(d.records, recordID)
	return nil
}

// EnsureBotDNS creates a DNS record for a bot (e.g., botname.spacename.hanzo.bot).
// Returns the fully qualified domain name.
func (d *DNSManager) EnsureBotDNS(ctx context.Context, spaceID, botID, botName string) (string, error) {
	// Sanitize the bot name for DNS.
	safeName := sanitizeDNSLabel(botName)
	if safeName == "" {
		safeName = botID
	}

	fqdn := fmt.Sprintf("%s.%s.hanzo.bot", safeName, spaceID)

	// Check if a record already exists for this bot.
	for _, r := range d.records {
		if r.SpaceID == spaceID && r.Name == fqdn && r.Managed {
			return fqdn, nil
		}
	}

	record := DNSRecord{
		ID:      uuid.New().String(),
		SpaceID: spaceID,
		Name:    fqdn,
		Type:    "CNAME",
		Value:   "playground.hanzo.ai",
		TTL:     300,
		Managed: true,
	}

	if err := d.CreateRecord(ctx, record); err != nil {
		return "", fmt.Errorf("failed to create bot dns record: %w", err)
	}

	return fqdn, nil
}

// sanitizeDNSLabel converts a human-readable name into a valid DNS label.
// Spaces and underscores become hyphens; invalid characters are dropped.
func sanitizeDNSLabel(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			b.WriteRune('-')
		}
	}
	// Collapse consecutive hyphens and trim.
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	result = strings.Trim(result, "-")
	if len(result) > 63 {
		result = result[:63]
	}
	return result
}
