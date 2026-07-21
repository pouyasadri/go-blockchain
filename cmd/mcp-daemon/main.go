package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pouyasadri/go-blockchain/internal/firewall"
	"github.com/pouyasadri/go-blockchain/internal/mcp"
)

func main() {
	policyPath := flag.String("policy", "policy.json", "Path to the signed policy.json file")
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.Printf("Starting MCP Daemon with policy %s", *policyPath)

	// 1. Load and cryptographically verify the policy
	// This will panic and lock the firewall if the signature is invalid or missing
	policy, err := firewall.LoadAndVerify(*policyPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to load policy: %v", err)
	}

	// 2. Initialize the financial firewall with the verified policy
	fw := firewall.NewFirewall(policy)
	log.Printf("Firewall initialized. Session budget: %d micro-cents", policy.SessionBudget)

	// 3. Initialize the MCP Daemon
	// We read from stdin, write JSON-RPC to stdout, and logs to stderr
	daemon, err := mcp.NewMCPDaemon(fw, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize MCP daemon: %v", err)
	}
	defer daemon.Close()

	// 4. Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal, shutting down...")
		cancel()
	}()

	// 5. Start the stdio JSON-RPC loop
	log.Println("MCP Daemon running and waiting for JSON-RPC 2.0 messages on stdin...")
	if err := daemon.Run(ctx); err != nil {
		log.Fatalf("Daemon exited with error: %v", err)
	}

	log.Println("MCP Daemon shut down cleanly")
}
