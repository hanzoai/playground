# Playground Go SDK

The Playground Go SDK provides idiomatic Go bindings for interacting with the Playground control plane.

## Installation

```bash
go get github.com/hanzoai/playground/sdk/go
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    playgroundbot "github.com/hanzoai/playground/sdk/go/bot"
)

func main() {
    bot, err := playgroundbot.New(playgroundbot.Config{
        NodeID:        "example-bot",
        Version:       "1.0.0",
        PlaygroundURL: "http://localhost:8080",
    })
    if err != nil {
        log.Fatal(err)
    }

    bot.RegisterReasoner("health", func(ctx context.Context, _ map[string]any) (any, error) {
        return map[string]any{"status": "ok"}, nil
    })

    if err := bot.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## Modules

- `bot`: Build Playground-compatible bots and register reasoners/skills.
- `client`: Low-level HTTP client for the Playground control plane.
- `types`: Shared data structures and contracts.
- `ai`: Helpers for interacting with AI providers via the control plane.

## Testing

```bash
go test ./...
```

## License

Distributed under the Apache 2.0 License. See the repository root for full details.
