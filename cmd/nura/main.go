package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/LawyZheng/nura/internal/gateway"
	"github.com/LawyZheng/nura/internal/pipeline"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe()
	case "ingest":
		cmdIngest()
	case "trace":
		cmdTrace()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: nura <command> [args]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  serve              Start the HTTP server")
	fmt.Fprintln(os.Stderr, "  ingest <file>      Ingest a report from file")
	fmt.Fprintln(os.Stderr, "  trace <id>         View an agent trace")
}

func cmdServe() {
	port := "8000"
	dataDir := defaultDataDir()

	// Parse optional flags.
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--port":
			if i+1 < len(os.Args) {
				port = os.Args[i+1]
				i++
			}
		case "--data-dir":
			if i+1 < len(os.Args) {
				dataDir = os.Args[i+1]
				i++
			}
		}
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fatalf("create data dir: %v", err)
	}

	s, llm, pe := initDeps(dataDir)
	defer s.Close()

	agent := runtime.NewAgentRuntime(llm, pe, s)
	srv := gateway.NewServer(agent, llm, s)

	// Graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nshutting down...")
		_ = srv.Shutdown(ctx)
		cancel()
	}()

	addr := ":" + port
	if err := srv.ListenAndServe(addr); err != nil && ctx.Err() == nil {
		fatalf("server: %v", err)
	}
}

func cmdIngest() {
	if len(os.Args) < 3 {
		fatalf("usage: nura ingest <file>")
	}

	filePath := os.Args[2]
	data, err := os.ReadFile(filePath)
	if err != nil {
		fatalf("read file: %v", err)
	}

	dataDir := defaultDataDir()
	s, llm, _ := initDeps(dataDir)
	defer s.Close()

	p := pipeline.NewPipeline(llm, s)
	result, err := p.Run(context.Background(), string(data), "2024-01-01")
	if err != nil {
		fatalf("ingest: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

func cmdTrace() {
	if len(os.Args) < 3 {
		fatalf("usage: nura trace <id>")
	}
	// Trace viewing requires a running server; in CLI mode, we just print a message.
	fmt.Fprintf(os.Stderr, "Trace viewing requires the server to be running.\n")
	fmt.Fprintf(os.Stderr, "Use: curl http://localhost:8000/agent/debug/trace/%s\n", os.Args[2])
}

func initDeps(dataDir string) (*store.Store, providers.LLMProvider, policy.PolicyEngine) {
	dbPath := filepath.Join(dataDir, "nura.db")
	s, err := store.New(dbPath)
	if err != nil {
		fatalf("init store: %v", err)
	}

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()

	return s, llm, pe
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nura"
	}
	return filepath.Join(home, ".nura")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "nura: "+format+"\n", args...)
	os.Exit(1)
}
