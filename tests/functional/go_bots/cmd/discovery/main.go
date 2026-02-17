package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/hanzoai/playground/sdk/go/bot"
)

func main() {
	log.SetFlags(0)

	baseURL := strings.TrimSpace(os.Getenv("AGENTS_URL"))
	if baseURL == "" {
		log.Fatal("AGENTS_URL is required")
	}

	nodeID := strings.TrimSpace(os.Getenv("AGENT_NODE_ID"))
	if nodeID == "" {
		nodeID = "go-sdk-discovery-client"
	}

	cfg := bot.Config{
		NodeID:        nodeID,
		Version:       "1.0.0",
		PlaygroundURL: baseURL,
		Token:         os.Getenv("AGENTS_TOKEN"),
	}

	client, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("init agent client: %v", err)
	}

	opts := buildDiscoveryOptionsFromEnv()
	result, err := client.Discover(context.Background(), opts...)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}

	output := map[string]interface{}{
		"format": result.Format,
	}

	switch result.Format {
	case "xml":
		output["xml"] = result.Raw
	case "compact":
		if result.Compact != nil {
			output["totals"] = map[string]int{
				"bots": len(result.Compact.Bots),
				"skills":    len(result.Compact.Skills),
			}
			output["bots"] = result.Compact.Bots
			output["skills"] = result.Compact.Skills
		}
	default:
		if result.JSON != nil {
			output["totals"] = map[string]int{
				"agents":    result.JSON.TotalAgents,
				"bots": result.JSON.TotalBots,
				"skills":    result.JSON.TotalSkills,
			}

			caps := make([]map[string]interface{}, 0, len(result.JSON.Capabilities))
			for _, cap := range result.JSON.Capabilities {
				caps = append(caps, map[string]interface{}{
					"agent_id":  cap.AgentID,
					"bots": cap.Bots,
					"skills":    cap.Skills,
				})
			}
			output["capabilities"] = caps
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		log.Fatalf("encode output: %v", err)
	}
}

func buildDiscoveryOptionsFromEnv() []bot.DiscoveryOption {
	var opts []bot.DiscoveryOption

	if v := strings.TrimSpace(os.Getenv("DISCOVERY_AGENT")); v != "" {
		opts = append(opts, bot.WithBot(v))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_NODE_ID")); v != "" {
		opts = append(opts, bot.WithNodeID(v))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_AGENT_IDS")); v != "" {
		opts = append(opts, bot.WithBotIDs(splitCSV(v)))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_NODE_IDS")); v != "" {
		opts = append(opts, bot.WithNodeIDs(splitCSV(v)))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_REASONER_PATTERN")); v != "" {
		opts = append(opts, bot.WithBotPattern(v))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_SKILL_PATTERN")); v != "" {
		opts = append(opts, bot.WithSkillPattern(v))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_TAGS")); v != "" {
		opts = append(opts, bot.WithTags(splitCSV(v)))
	}

	if parseEnvBool("DISCOVERY_INCLUDE_INPUT_SCHEMA") {
		opts = append(opts, bot.WithDiscoveryInputSchema(true))
	}
	if parseEnvBool("DISCOVERY_INCLUDE_OUTPUT_SCHEMA") {
		opts = append(opts, bot.WithDiscoveryOutputSchema(true))
	}
	if val, ok := parseEnvBoolStrict("DISCOVERY_INCLUDE_DESCRIPTIONS"); ok {
		opts = append(opts, bot.WithDiscoveryDescriptions(val))
	}
	if val, ok := parseEnvBoolStrict("DISCOVERY_INCLUDE_EXAMPLES"); ok {
		opts = append(opts, bot.WithDiscoveryExamples(val))
	}

	if v := strings.TrimSpace(os.Getenv("DISCOVERY_FORMAT")); v != "" {
		opts = append(opts, bot.WithFormat(v))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_HEALTH_STATUS")); v != "" {
		opts = append(opts, bot.WithHealthStatus(v))
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_LIMIT")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			opts = append(opts, bot.WithLimit(parsed))
		}
	}
	if v := strings.TrimSpace(os.Getenv("DISCOVERY_OFFSET")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			opts = append(opts, bot.WithOffset(parsed))
		}
	}
	return opts
}

func parseEnvBool(name string) bool {
	val, ok := parseEnvBoolStrict(name)
	return ok && val
}

func parseEnvBoolStrict(name string) (bool, bool) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return false, false
	}
	switch strings.ToLower(raw) {
	case "true", "1", "yes", "y":
		return true, true
	case "false", "0", "no", "n":
		return false, true
	default:
		return false, false
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	results := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			results = append(results, part)
		}
	}
	return results
}
