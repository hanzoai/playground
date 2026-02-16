package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hanzoai/playground/sdk/go/bot"
)

func main() {
	nodeID := strings.TrimSpace(os.Getenv("HANZO_NODE_ID"))
	if nodeID == "" {
		nodeID = "my-bot"
	}

	playgroundURL := strings.TrimSpace(os.Getenv("PLAYGROUND_URL"))
	listenAddr := strings.TrimSpace(os.Getenv("HANZO_LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = ":8001"
	}

	publicURL := strings.TrimSpace(os.Getenv("HANZO_PUBLIC_URL"))
	if publicURL == "" {
		publicURL = "http://localhost" + listenAddr
	}

	cfg := bot.Config{
		NodeID:        nodeID,
		Version:       "1.0.0",
		PlaygroundURL: playgroundURL, // optional for CLI-only
		Token:         os.Getenv("PLAYGROUND_TOKEN"),
		ListenAddress: listenAddr,
		PublicURL:     publicURL,
		CLIConfig: &bot.CLIConfig{
			AppName:        "go-bot-hello",
			AppDescription: "Go SDK hello-world with CLI + control plane",
			HelpPreamble:   "Pass --set message=YourName to customize the greeting.",
			EnvironmentVars: []string{
				"PLAYGROUND_URL (optional) Control plane URL for server mode",
				"PLAYGROUND_TOKEN (optional) Bearer token",
				"HANZO_NODE_ID (optional) Override node id (default: my-bot)",
			},
		},
	}

	hello, err := bot.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	hasControlPlane := strings.TrimSpace(cfg.PlaygroundURL) != ""

	addEmojiLocal := func(message string) map[string]any {
		trimmed := strings.TrimSpace(message)
		if trimmed == "" {
			trimmed = "Hello!"
		}
		return map[string]any{
			"text":  trimmed,
			"emoji": "👋",
		}
	}

	hello.RegisterBot("add_emoji", func(ctx context.Context, input map[string]any) (any, error) {
		msg := fmt.Sprintf("%v", input["message"])
		return addEmojiLocal(msg), nil
	},
		bot.WithDescription("Adds a friendly emoji to a message"),
	)

	hello.RegisterBot("say_hello", func(ctx context.Context, input map[string]any) (any, error) {
		name := strings.TrimSpace(fmt.Sprintf("%v", input["name"]))
		if name == "" || name == "<nil>" {
			name = "World"
		}
		greeting := fmt.Sprintf("Hello, %s!", name)

		var decorated map[string]any
		if hasControlPlane {
			// Prefer control plane call so workflow edges are captured.
			res, callErr := hello.Call(ctx, "add_emoji", map[string]any{"message": greeting})
			if callErr == nil {
				decorated = res
			} else {
				log.Printf("warn: control plane call to add_emoji failed: %v", callErr)
			}
		}
		if decorated == nil {
			decorated = addEmojiLocal(greeting)
		}

		return map[string]any{
			"greeting": fmt.Sprintf("%s %s", decorated["text"], decorated["emoji"]),
			"name":     name,
		}, nil
	},
		bot.WithCLI(),
		bot.WithDescription("Greets a user, enriching the message via add_emoji"),
	)

	hello.RegisterBot("demo_echo", func(ctx context.Context, input map[string]any) (any, error) {
		message := strings.TrimSpace(fmt.Sprintf("%v", input["message"]))
		if message == "" || message == "<nil>" {
			message = "Playground"
		}

		if hasControlPlane {
			res, callErr := hello.Call(ctx, "say_hello", map[string]any{"name": message})
			if callErr == nil {
				return res, nil
			}
			log.Printf("warn: control plane call to say_hello failed: %v", callErr)
		}

		return hello.Execute(ctx, "say_hello", map[string]any{"name": message})
	},
		bot.WithCLI(),
		bot.WithDefaultCLI(),
		bot.WithDescription("Echo entry point that chains into say_hello -> add_emoji"),
		bot.WithCLIFormatter(func(ctx context.Context, result any, err error) {
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			if resMap, ok := result.(map[string]any); ok {
				fmt.Printf("%s (%s)\n", resMap["greeting"], resMap["name"])
				return
			}
			fmt.Println(result)
		}),
	)

	if err := hello.Run(context.Background()); err != nil {
		if cliErr, ok := err.(*bot.CLIError); ok {
			os.Exit(cliErr.ExitCode())
		}
		log.Fatal(err)
	}
}
