# Product Requirements Document: Go SDK CLI Integration

**Version:** 1.0
**Last Updated:** November 22, 2025
**Status:** Draft
**Owner:** AgentField SDK Team

---

## Executive Summary

This PRD outlines the design and implementation of native CLI capabilities for the AgentField Go SDK. The goal is to provide developers with a zero-boilerplate experience where compiled agent binaries automatically function as both server nodes (connected to the control plane) and standalone CLI tools with full AI capabilities.

---

## Problem Statement

### Current State
Currently, developers using the AgentField Go SDK must:
- Write explicit server code to expose their agents as network services
- Manually handle command-line argument parsing if they want CLI capabilities
- Choose between control plane integration OR standalone execution
- Implement custom output formatting for CLI use cases
- Maintain separate codebases for server and CLI modes

### Pain Points
1. **High boilerplate**: Developers need to write significant infrastructure code
2. **Mode switching complexity**: No easy way to toggle between server and CLI modes
3. **Developer experience**: Poor ergonomics compared to modern CLI frameworks
4. **Output customization**: No built-in way to format output for human-readable CLI display
5. **Testing friction**: Difficult to test reasoners locally without running the full control plane

---

## Goals and Non-Goals

### Goals
1. **Zero-boilerplate CLI**: Compiled binaries automatically gain CLI capabilities
2. **Dual-mode operation**: Single binary supports both `serve` (control plane) and standalone execution
3. **Flexible input handling**: Support multiple input formats (flags, JSON, files)
4. **CLI-optimized output**: Allow developers to customize output formatting for CLI contexts
5. **Backward compatibility**: Existing code continues to work without modification
6. **Developer experience**: World-class DX comparable to best-in-class CLI frameworks

### Non-Goals
1. Complex CLI framework features (plugins, aliases, advanced scripting)
2. Interactive REPL mode (may be added in future iterations)
3. Breaking changes to existing SDK APIs
4. Support for languages other than Go (this PRD is Go-specific)

---

## User Stories

### Story 1: Developer - Quick Testing
**As a** developer building an agent
**I want to** test my reasoners locally without starting the control plane
**So that** I can iterate faster during development

```bash
# Quick test of a reasoner
./myagent greet --set name=Alice --set greeting="Hello"
```

### Story 2: Developer - Production Server
**As a** developer deploying an agent
**I want to** run my compiled binary as a server connected to the control plane
**So that** it can participate in the AgentField ecosystem

```bash
# Production deployment
./myagent serve
```

### Story 3: Developer - CLI Distribution
**As a** developer building tools
**I want to** distribute my agent as a CLI tool to end users
**So that** they can use AI capabilities without running a server

```bash
# End user executes as CLI tool
./myagent analyze --input-file document.pdf --set format=summary
```

### Story 4: Developer - Custom Output
**As a** developer
**I want to** control how my reasoner outputs are displayed in CLI mode
**So that** users get human-friendly formatted results

```go
agent.RegisterReasoner("greet", greetHandler,
    agent.WithCLIFormatter(func(ctx context.Context, result any, err error) {
        fmt.Printf("✓ %v\n", result)
    }))
```

### Story 5: Operations - List Capabilities
**As an** operator or end user
**I want to** see what reasoners are available in a compiled binary
**So that** I understand what commands I can execute

```bash
./myagent list
# Output:
# Available reasoners:
#   greet (default) - Greets a user
#   analyze - Analyzes documents
#   summarize - Summarizes text
```

---

## Technical Requirements

### Functional Requirements

#### FR-1: Automatic CLI Mode Detection
- The SDK MUST automatically detect if the binary is invoked as CLI or server
- Server mode is triggered by the `serve` command
- CLI mode is triggered by any other command or flags

#### FR-2: Reasoner CLI Registration
- Developers MUST be able to mark reasoners as CLI-accessible via `WithCLI()` option
- Developers MUST be able to designate one reasoner as default via `WithDefaultCLI()` option
- Non-CLI-marked reasoners should not be exposed in CLI mode

#### FR-3: Input Handling
The SDK MUST support three input methods:

1. **Individual flags** (`--set key=value`):
   ```bash
   ./agent greet --set name=Alice --set age=30
   ```

2. **JSON string** (`--input`):
   ```bash
   ./agent greet --input '{"name":"Alice","age":30}'
   ```

3. **JSON file** (`--input-file`):
   ```bash
   ./agent greet --input-file user.json
   ```

4. **Stdin** (implicit):
   ```bash
   echo '{"name":"Alice"}' | ./agent greet
   ```

**Merging priority**: `--set` > `--input` > `--input-file` > stdin

#### FR-4: Output Customization
- Developers MUST be able to register custom CLI formatters via `WithCLIFormatter()`
- Default output format MUST be pretty-printed JSON
- Formatters receive `(ctx, result, error)` and have full control over stdout/stderr

#### FR-5: CLI Context Detection
- Reasoner handlers MUST be able to detect CLI mode via `agent.IsCLIMode(ctx)`
- Context allows conditional behavior (e.g., progress bars in CLI, JSON in server)

#### FR-6: Server Mode
- `serve` command MUST start HTTP server and register with control plane
- All existing `Run()` functionality MUST be preserved
- Server mode MUST validate required configuration (AgentFieldURL)

#### FR-7: Built-in Commands
The SDK MUST provide these built-in commands:
- `serve` - Start agent server (control plane mode)
- `list` - List all CLI-accessible reasoners
- `help` - Display usage information
- `version` - Display SDK/agent version

### Non-Functional Requirements

#### NFR-1: Performance
- CLI startup time MUST be < 100ms (excluding reasoner execution)
- No performance degradation in server mode

#### NFR-2: Backward Compatibility
- Existing `agent.Run(ctx)` calls MUST continue to work
- New CLI features are opt-in via `WithCLI()` options

#### NFR-3: Error Handling
- Clear error messages for invalid input
- Proper exit codes (0 = success, 1 = error, 2 = usage error)
- Errors written to stderr, not stdout

#### NFR-4: Dependencies
- MUST use only Go stdlib (no external CLI framework dependencies)
- Keep SDK lightweight and minimal

---

## API Design

### 1. Reasoner Registration Options

```go
type ReasonerOption func(*Reasoner)

// WithCLI marks this reasoner as CLI-accessible
func WithCLI() ReasonerOption

// WithDefaultCLI marks this as the default reasoner for CLI invocation
func WithDefaultCLI() ReasonerOption

// WithCLIFormatter provides custom output formatting for CLI mode
func WithCLIFormatter(formatter func(context.Context, any, error)) ReasonerOption

// WithDescription adds help text for the reasoner
func WithDescription(desc string) ReasonerOption

// Existing options
func WithInputSchema(raw json.RawMessage) ReasonerOption
func WithOutputSchema(raw json.RawMessage) ReasonerOption
```

### 2. Context Helpers

```go
// IsCLIMode returns true if the current execution is in CLI mode
func IsCLIMode(ctx context.Context) bool

// GetCLIArgs returns parsed CLI arguments (when in CLI mode)
func GetCLIArgs(ctx context.Context) map[string]string
```

### 3. Agent Configuration

```go
type Config struct {
    NodeID        string
    Version       string
    TeamID        string
    AgentFieldURL string // Optional for CLI-only usage
    ListenAddress string
    PublicURL     string
    Token         string

    LeaseRefreshInterval time.Duration
    DisableLeaseLoop     bool
    Logger               *log.Logger
    AIConfig             *ai.Config

    // New: CLI configuration
    CLIConfig *CLIConfig
}

type CLIConfig struct {
    // AppName for help text (defaults to binary name)
    AppName string

    // AppDescription for help text
    AppDescription string

    // DisableColors turns off ANSI colors in output
    DisableColors bool

    // DefaultOutputFormat: "json", "pretty", "yaml"
    DefaultOutputFormat string

    // HelpPreamble shown before usage information
    HelpPreamble string

    // HelpEpilog shown after usage information
    HelpEpilog string

    // EnvironmentVars list of environment variables to document in help
    EnvironmentVars []string
}
```

### 4. Modified Agent Methods

```go
// Run intelligently handles both CLI and server modes
// Inspects os.Args to determine mode
func (a *Agent) Run(ctx context.Context) error

// Serve explicitly starts server mode (can be called directly)
func (a *Agent) Serve(ctx context.Context) error

// Execute runs a specific reasoner (used internally by CLI mode)
func (a *Agent) Execute(ctx context.Context, reasonerName string, input map[string]any) (any, error)
```

---

## Implementation Plan

### Phase 1: Core CLI Infrastructure (Week 1)

**Tasks:**
1. Add `CLIMode`, `DefaultCLI`, `CLIFormatter`, `Description` fields to `Reasoner` struct
2. Implement `WithCLI()`, `WithDefaultCLI()`, `WithCLIFormatter()`, `WithDescription()` options
3. Create `cli.go` file with argument parsing logic
4. Implement `parseArgs()` function supporting `--set`, `--input`, `--input-file`
5. Modify `New()` to make `AgentFieldURL` optional

**Deliverables:**
- [ ] `sdk/go/agent/cli.go` with parsing logic
- [ ] Unit tests for argument parsing
- [ ] Updated `agent.go` with new option functions

### Phase 2: Mode Detection and Routing (Week 1)

**Tasks:**
1. Refactor `Run()` to inspect `os.Args[1]`
2. Implement routing logic: `serve`, `list`, `help`, `version`, `<reasoner>`
3. Create `Serve()` method (rename current `Run()` internals)
4. Implement `Execute()` method for direct reasoner invocation
5. Add context values for CLI mode detection

**Deliverables:**
- [ ] Refactored `Run()` method
- [ ] New `Serve()` method
- [ ] `IsCLIMode()` helper function
- [ ] Integration tests

### Phase 3: Output Formatting (Week 2)

**Tasks:**
1. Implement default JSON pretty-printer
2. Wire up `CLIFormatter` execution
3. Add color support (respecting `CLIConfig.DisableColors`)
4. Implement help text generation
5. Add `list` command implementation

**Deliverables:**
- [ ] Default formatters
- [ ] Help text generation
- [ ] `list` command
- [ ] Examples with custom formatters

### Phase 4: Documentation and Examples (Week 2)

**Tasks:**
1. Update SDK README with CLI examples
2. Create example agents demonstrating CLI usage
3. Write migration guide for existing users
4. Add API documentation
5. Create video tutorial

**Deliverables:**
- [ ] Updated `sdk/go/README.md`
- [ ] `examples/go/cli-agent/` example
- [ ] Migration guide document
- [ ] API reference docs

### Phase 5: Testing and Polish (Week 3)

**Tasks:**
1. Comprehensive integration tests
2. Error message polish
3. Performance testing
4. Cross-platform testing (Linux, macOS, Windows)
5. Community feedback incorporation

**Deliverables:**
- [ ] Test coverage > 80%
- [ ] Benchmark results
- [ ] Cross-platform verification
- [ ] Beta release

---

## Usage Examples

### Example 1: Basic Agent with CLI

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Agent-Field/agentfield/sdk/go/agent"
)

func main() {
    a, err := agent.New(agent.Config{
        NodeID:        "greeting-agent",
        Version:       "1.0.0",
        AgentFieldURL: "http://localhost:8080", // Optional for CLI-only
    })
    if err != nil {
        log.Fatal(err)
    }

    // Register reasoner with CLI support
    a.RegisterReasoner("greet", func(ctx context.Context, input map[string]any) (any, error) {
        name := input["name"].(string)
        greeting := fmt.Sprintf("Hello, %s!", name)

        // Detect CLI mode for custom behavior
        if agent.IsCLIMode(ctx) {
            return greeting, nil
        }

        return map[string]any{"greeting": greeting}, nil
    },
        agent.WithCLI(),
        agent.WithDefaultCLI(),
        agent.WithDescription("Greets a user by name"),
    )

    // Single entry point handles everything
    if err := a.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

**Usage:**
```bash
# Build
go build -o myagent

# CLI mode (uses default reasoner)
./myagent --set name=Alice
# Output: Hello, Alice!

# CLI mode (explicit reasoner)
./myagent greet --set name=Bob
# Output: Hello, Bob!

# Server mode
./myagent serve
# Output: [agent] listening on :8001
```

### Example 2: Agent with Custom Formatter

```go
a.RegisterReasoner("analyze", analyzeHandler,
    agent.WithCLI(),
    agent.WithDescription("Analyzes text and provides insights"),
    agent.WithCLIFormatter(func(ctx context.Context, result any, err error) {
        if err != nil {
            fmt.Fprintf(os.Stderr, "❌ Analysis failed: %v\n", err)
            os.Exit(1)
        }

        data := result.(map[string]any)
        fmt.Println("📊 Analysis Results")
        fmt.Println("──────────────────")
        fmt.Printf("Sentiment: %s\n", data["sentiment"])
        fmt.Printf("Confidence: %.2f%%\n", data["confidence"].(float64)*100)
        fmt.Printf("Key topics: %v\n", data["topics"])
    }),
)
```

**Usage:**
```bash
./myagent analyze --input-file document.txt
# Output:
# 📊 Analysis Results
# ──────────────────
# Sentiment: positive
# Confidence: 87.50%
# Key topics: [technology innovation growth]
```

### Example 3: Multi-Input Methods

```bash
# Method 1: Individual flags
./myagent search --set query="AI agents" --set limit=10 --set format=json

# Method 2: JSON input
./myagent search --input '{"query":"AI agents","limit":10,"format":"json"}'

# Method 3: JSON file
cat search.json
# {"query":"AI agents","limit":10,"format":"json"}
./myagent search --input-file search.json

# Method 4: Stdin
echo '{"query":"AI agents"}' | ./myagent search

# Method 5: Combining (--set overrides)
./myagent search --input-file base.json --set limit=100
```

### Example 4: AI-Powered CLI Tool with Custom Help

```go
a, err := agent.New(agent.Config{
    NodeID:  "ai-assistant",
    Version: "1.0.0",
    AIConfig: &ai.Config{
        Provider: "openai",
        APIKey:   os.Getenv("OPENAI_API_KEY"),
        Model:    "gpt-4",
    },
    CLIConfig: &agent.CLIConfig{
        AppName:        "AI Assistant",
        AppDescription: "Intelligent command-line assistant powered by GPT-4",

        HelpPreamble: `
⚠️  IMPORTANT: Set OPENAI_API_KEY before running.
For best results, use GPT-4 model (default).`,

        EnvironmentVars: []string{
            "OPENAI_API_KEY (required) - Your OpenAI API key",
            "MODEL (optional) - AI model to use (default: gpt-4)",
            "TEMPERATURE (optional) - Response creativity 0.0-2.0 (default: 0.7)",
        },

        HelpEpilog: `
Examples:
  $ export OPENAI_API_KEY=sk-...
  $ ./ai-assistant --set question="Explain quantum computing"
  $ ./ai-assistant ask --set question="Write a poem" --set temperature=1.5

For more information: https://docs.example.com/ai-assistant`,
    },
})

a.RegisterReasoner("ask", func(ctx context.Context, input map[string]any) (any, error) {
    question := input["question"].(string)

    // Use AI capability
    resp, err := a.AI(ctx, question)
    if err != nil {
        return nil, err
    }

    return resp.Choices[0].Message.Content, nil
},
    agent.WithCLI(),
    agent.WithDefaultCLI(),
    agent.WithDescription("Ask the AI a question"),
    agent.WithCLIFormatter(func(ctx context.Context, result any, err error) {
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            return
        }
        fmt.Println(result)
    }),
)
```

**Usage:**
```bash
# Run the assistant
./ai-assistant --set question="What is quantum computing?"
# Output: Quantum computing is a type of computation that...

# View help with custom sections
./ai-assistant --help
# Output:
# AI Assistant - Intelligent command-line assistant powered by GPT-4
#
# ⚠️  IMPORTANT: Set OPENAI_API_KEY before running.
# For best results, use GPT-4 model (default).
#
# Usage:
#   ai-assistant [command] [flags]
#
# Available Commands:
#   ask (default)  Ask the AI a question
#   serve          Start agent server
#   list           List available reasoners
#   help           Show help information
#
# Environment Variables:
#   OPENAI_API_KEY (required) - Your OpenAI API key
#   MODEL (optional) - AI model to use (default: gpt-4)
#   TEMPERATURE (optional) - Response creativity 0.0-2.0 (default: 0.7)
#
# Examples:
#   $ export OPENAI_API_KEY=sk-...
#   $ ./ai-assistant --set question="Explain quantum computing"
#   $ ./ai-assistant ask --set question="Write a poem" --set temperature=1.5
#
# For more information: https://docs.example.com/ai-assistant
```

---

## Command Reference

### `serve`
Start the agent as a server and connect to the control plane.

**Usage:**
```bash
./agent serve
```

**Environment Variables:**
- `AGENTFIELD_URL` - Override control plane URL
- `AGENTFIELD_TOKEN` - Authentication token
- `LISTEN_ADDRESS` - Server listen address (default: `:8001`)

### `<reasoner>` or default
Execute a reasoner in standalone CLI mode.

**Usage:**
```bash
./agent [reasoner-name] [flags]
```

**Flags:**
- `--set key=value` - Set individual input parameters (repeatable)
- `--input <json>` - Provide input as JSON string
- `--input-file <path>` - Load input from JSON file
- `--output <format>` - Output format: json, pretty, yaml (default: pretty)
- `--no-color` - Disable colored output

**Examples:**
```bash
# Use default reasoner
./agent --set name=Alice

# Use specific reasoner
./agent greet --set name=Bob

# Multiple inputs
./agent process --set input="data" --set format="json" --set verbose=true
```

### `list`
List all CLI-accessible reasoners.

**Usage:**
```bash
./agent list
```

**Output:**
```
Available reasoners:
  greet (default) - Greets a user by name
  analyze - Analyzes text and provides insights
  summarize - Summarizes long documents
```

### `help`
Display help information.

**Usage:**
```bash
./agent help
./agent help <reasoner>
```

### `version`
Display version information.

**Usage:**
```bash
./agent version
```

**Output:**
```
AgentField SDK: v1.0.0
Agent: greeting-agent v1.0.0
Go: go1.21
```

---

## Success Metrics

### Developer Experience Metrics
- **Time to first CLI execution**: < 5 minutes from SDK install
- **Lines of boilerplate code**: 0 lines required for basic CLI
- **Onboarding friction**: Measured via developer survey

### Technical Metrics
- **CLI startup time**: < 100ms
- **Memory overhead**: < 10MB additional for CLI mode
- **Test coverage**: > 80%
- **Documentation completeness**: 100% of public APIs documented

### Adoption Metrics
- **Usage rate**: % of Go SDK users enabling CLI features (target: 40% in 6 months)
- **GitHub stars/forks**: Track community interest
- **Support tickets**: Reduction in CLI-related questions (target: -50%)

---

## Security Considerations

1. **Input Validation**
   - All CLI inputs must go through same validation as API inputs
   - Prevent command injection via `--set` flags
   - Sanitize file paths for `--input-file`

2. **Credential Handling**
   - Support environment variables for sensitive config
   - Never log credentials or tokens
   - Warn if tokens appear in command history

3. **Error Messages**
   - Don't leak sensitive information in error messages
   - Sanitize stack traces in production builds

---

## Open Questions

1. **Q:** Should we support shell completion (bash/zsh)?
   **A:** Post-MVP feature, track as future enhancement

2. **Q:** Should we support config files (e.g., `.agentrc`)?
   **A:** Post-MVP, but design API to be config-file friendly

3. **Q:** How to handle long-running CLI operations?
   **A:** Support progress bars via CLI context, allow cancellation via Ctrl+C

4. **Q:** Should reasoners be able to read from stdin *and* flags simultaneously?
   **A:** Yes, merge all inputs (flags override stdin)

5. **Q:** How to handle binary/non-JSON data in CLI mode?
   **A:** Support `--input-binary <file>` for binary inputs, encode as base64 in input map

---

## Timeline

- **Week 1**: Core CLI infrastructure + mode detection
- **Week 2**: Output formatting + documentation
- **Week 3**: Testing + polish
- **Week 4**: Beta release + community feedback
- **Week 5**: GA release

---

## Appendix A: Alternative Designs Considered

### Alternative 1: Separate CLI Framework
Use a framework like Cobra or urfave/cli.

**Pros:** Rich features, battle-tested
**Cons:** Additional dependency, steeper learning curve, more boilerplate
**Decision:** Rejected - conflicts with zero-boilerplate goal

### Alternative 2: Code Generation
Generate CLI code from reasoner definitions.

**Pros:** Type-safe, fast
**Cons:** Build step complexity, tooling requirements
**Decision:** Rejected - adds too much complexity

### Alternative 3: Struct Tags for CLI Params
Use struct tags to define CLI parameters.

```go
type Input struct {
    Name string `cli:"name,required"`
    Age  int    `cli:"age"`
}
```

**Pros:** Type-safe, declarative
**Cons:** Requires structured inputs, less flexible
**Decision:** Rejected - reasoners already use `map[string]any`

---

## Appendix B: Migration Guide

### For Existing Users

**Before:**
```go
func main() {
    agent, _ := agent.New(config)
    agent.RegisterReasoner("greet", handler)
    agent.Run(context.Background())
}
```

**After (no changes required, but can add CLI):**
```go
func main() {
    agent, _ := agent.New(config)
    agent.RegisterReasoner("greet", handler,
        agent.WithCLI(),  // Enable CLI - OPTIONAL
    )
    agent.Run(context.Background())  // Works as before
}
```

**Breaking Changes:** None

**Deprecations:** None

---

## Appendix C: Future Enhancements

1. **Interactive REPL Mode**
   ```bash
   ./agent repl
   > greet name=Alice
   Hello, Alice!
   > analyze --input-file doc.txt
   ...
   ```

2. **Shell Completion**
   ```bash
   ./agent completion bash > /etc/bash_completion.d/agent
   ```

3. **Plugin System**
   ```bash
   ./agent plugin install github.com/user/plugin
   ```

4. **Built-in Debugging**
   ```bash
   ./agent greet --debug --set name=Alice
   ```

5. **Configuration Profiles**
   ```bash
   ./agent --profile production greet --set name=Alice
   ```

---

## References

- [Go SDK Documentation](../sdk/go/README.md)
- [AgentField Architecture](./ARCHITECTURE.md)
- [CLI Best Practices](https://clig.dev/)
- [12 Factor CLI Apps](https://medium.com/@jdxcode/12-factor-cli-apps-dd3c227a0e46)

---

**Document Status:** ✅ Ready for Review
**Next Review Date:** TBD
**Approvers:** SDK Team Lead, Product Manager, Engineering Manager
