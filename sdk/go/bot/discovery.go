package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hanzoai/playground/sdk/go/types"
)

// DiscoveryOption configures discovery requests.
type DiscoveryOption func(*discoveryOptions)

type discoveryOptions struct {
	botIDs              []string
	botPattern     string
	skillPattern        string
	tags                []string
	includeInput        bool
	includeOutput       bool
	includeDescriptions *bool
	includeExamples     *bool
	format              string
	healthStatus        string
	limit               *int
	offset              *int
}

// WithBot filters discovery to a single bot ID.
func WithBot(id string) DiscoveryOption {
	return func(o *discoveryOptions) {
		if id != "" {
			o.botIDs = append(o.botIDs, id)
		}
	}
}

// WithNodeID aliases WithBot for clarity.
func WithNodeID(id string) DiscoveryOption {
	return WithBot(id)
}

// WithBotIDs filters discovery to a set of bot IDs.
func WithBotIDs(ids []string) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.botIDs = append(o.botIDs, ids...)
	}
}

// WithNodeIDs aliases WithBotIDs.
func WithNodeIDs(ids []string) DiscoveryOption {
	return WithBotIDs(ids)
}

// WithBotPattern applies a wildcard pattern to bot IDs.
func WithBotPattern(pattern string) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.botPattern = pattern
	}
}

// WithSkillPattern applies a wildcard pattern to skill IDs.
func WithSkillPattern(pattern string) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.skillPattern = pattern
	}
}

// WithTags filters capabilities by tag (supports wildcards).
func WithTags(tags []string) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.tags = append(o.tags, tags...)
	}
}

// WithDiscoveryInputSchema toggles inclusion of input schemas.
func WithDiscoveryInputSchema(enabled bool) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.includeInput = enabled
	}
}

// WithDiscoveryOutputSchema toggles inclusion of output schemas.
func WithDiscoveryOutputSchema(enabled bool) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.includeOutput = enabled
	}
}

// WithDiscoveryDescriptions toggles inclusion of descriptions.
func WithDiscoveryDescriptions(enabled bool) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.includeDescriptions = &enabled
	}
}

// WithDiscoveryExamples toggles inclusion of examples.
func WithDiscoveryExamples(enabled bool) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.includeExamples = &enabled
	}
}

// WithFormat sets the desired response format: json (default), xml, or compact.
func WithFormat(format string) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.format = strings.ToLower(format)
	}
}

// WithHealthStatus filters by health status.
func WithHealthStatus(status string) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.healthStatus = strings.ToLower(status)
	}
}

// WithLimit controls pagination limit.
func WithLimit(limit int) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.limit = &limit
	}
}

// WithOffset controls pagination offset.
func WithOffset(offset int) DiscoveryOption {
	return func(o *discoveryOptions) {
		o.offset = &offset
	}
}

// Discover queries the control plane discovery API.
func (b *Bot) Discover(ctx context.Context, opts ...DiscoveryOption) (*types.DiscoveryResult, error) {
	if strings.TrimSpace(b.cfg.PlaygroundURL) == "" {
		return nil, fmt.Errorf("PlaygroundURL is required for discovery")
	}

	options := discoveryOptions{format: "json"}
	for _, opt := range opts {
		opt(&options)
	}
	if options.format == "" {
		options.format = "json"
	}

	switch options.format {
	case "json", "xml", "compact":
	default:
		return nil, fmt.Errorf("invalid discovery format: %s", options.format)
	}

	params := url.Values{}
	agents := dedupe(options.botIDs)
	switch len(agents) {
	case 0:
	case 1:
		params.Set("agent", agents[0])
	default:
		params.Set("agent_ids", strings.Join(agents, ","))
	}

	if options.botPattern != "" {
		params.Set("bot", options.botPattern)
	}
	if options.skillPattern != "" {
		params.Set("skill", options.skillPattern)
	}

	if len(options.tags) > 0 {
		params.Set("tags", strings.Join(dedupe(options.tags), ","))
	}
	if options.includeInput {
		params.Set("include_input_schema", "true")
	}
	if options.includeOutput {
		params.Set("include_output_schema", "true")
	}
	if options.includeDescriptions != nil {
		params.Set("include_descriptions", strconv.FormatBool(*options.includeDescriptions))
	}
	if options.includeExamples != nil {
		params.Set("include_examples", strconv.FormatBool(*options.includeExamples))
	}
	if options.healthStatus != "" {
		params.Set("health_status", options.healthStatus)
	}
	if options.limit != nil {
		params.Set("limit", strconv.Itoa(*options.limit))
	}
	if options.offset != nil {
		params.Set("offset", strconv.Itoa(*options.offset))
	}
	params.Set("format", options.format)

	endpoint := strings.TrimSuffix(b.cfg.PlaygroundURL, "/") + "/api/v1/discovery/capabilities"
	requestURL := endpoint
	if encoded := params.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build discovery request: %w", err)
	}
	if options.format == "xml" {
		req.Header.Set("Accept", "application/xml")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if b.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform discovery request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read discovery response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("discovery request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	result := &types.DiscoveryResult{
		Format: options.format,
		Raw:    string(body),
	}

	switch options.format {
	case "xml":
		result.XML = string(body)
	case "compact":
		var compact types.CompactDiscoveryResponse
		if err := json.Unmarshal(body, &compact); err != nil {
			return nil, fmt.Errorf("decode compact discovery response: %w", err)
		}
		result.Compact = &compact
	default:
		var full types.DiscoveryResponse
		if err := json.Unmarshal(body, &full); err != nil {
			return nil, fmt.Errorf("decode discovery response: %w", err)
		}
		result.JSON = &full
	}

	return result, nil
}

func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
