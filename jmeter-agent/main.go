package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"jmeter-agent/network"
)

func main() {
	// 1. Initialize Logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("Starting JMeter Agent...")

	// 2. Parse Command Line Flags
	agentID := flag.String("id", "", "The Agent ID (defaults to hostname)")
	hubURL := flag.String("hub", "ws://localhost:8080/ws", "The WebSocket URL of the JMeter Hub")
	flag.Parse()

	if *hubURL == "" {
		slog.Error("Hub URL must be provided via -hub flag")
		os.Exit(1)
	}

	id := *agentID
	if id == "" {
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			// Append a short random string to the hostname
			// to allow multiple agents on the same host default to unique IDs
			randomBytes := make([]byte, 2)
			if _, err := rand.Read(randomBytes); err == nil {
				id = fmt.Sprintf("%s-%x", hostname, randomBytes)
			} else {
				id = hostname // Fallback
			}
		} else {
			id = "agent-unknown"
		}
	}

	finalURL := *hubURL
	if !strings.Contains(finalURL, "agentId=") {
		if !strings.Contains(finalURL, "?") {
			finalURL += "?agentId=" + id
		} else {
			finalURL += "&agentId=" + id
		}
	}

	// 3. Connect to the Hub
	client := network.NewClient(finalURL, id)
	if err := client.Connect(); err != nil {
		slog.Error("Initial connection to Hub failed. Will retry in background.", "error", err)
	}

	// 4. Start the listener routine
	go client.Listen()

	// 5. Keep the agent running until interrupted
	slog.Info("Agent is running and waiting for commands...", "hub_url", *hubURL)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Shutting down JMeter Agent...")
	client.Close()
}
