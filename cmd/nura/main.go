package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/LawyZheng/nura/internal/config"
	"github.com/LawyZheng/nura/internal/gateway"
	"github.com/LawyZheng/nura/internal/logging"
	"github.com/LawyZheng/nura/internal/pipeline"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
)

var (
	cfgFile string
	cfg     *config.Config
	logger  *zap.Logger
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "nura",
		Short: "Nura - local-first health AI agent",
		Long:  "Nura (知愈) is a local-first health AI agent that ingests medical reports, runs an AI agent loop, and serves results via HTTP.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "version" {
				return nil
			}

			var err error
			cfg, err = config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger, err = logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}

			return nil
		},
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")

	root.AddCommand(newServeCmd())
	root.AddCommand(newIngestCmd())
	root.AddCommand(newTraceCmd())
	root.AddCommand(newVersionCmd())

	return root
}

func newServeCmd() *cobra.Command {
	var (
		port     int
		dataDir  string
		provider string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Long:  "Start the Nura HTTP gateway that exposes the agent API, report ingestion, and trace debugging endpoints.",
		Example: `  nura serve
  nura serve --port 9090
  nura serve --provider anthropic
  nura serve --port 9090 --data-dir /tmp/nura-data
  nura serve --config nura.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("port") {
				port = cfg.Server.Port
			}
			if !cmd.Flags().Changed("data-dir") {
				dataDir = cfg.Server.DataDir
			}
			if cmd.Flags().Changed("provider") {
				cfg.LLM.Provider = provider
			}

			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return fmt.Errorf("create data dir: %w", err)
			}

			logger.Info("starting nura server",
				zap.Int("port", port),
				zap.String("data_dir", dataDir),
			)

			s, llm, pe := initDeps(dataDir)
			defer s.Close()

			agent := runtime.NewAgentRuntime(llm, pe, s)
			srv := gateway.NewServer(agent, llm, s)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				logger.Info("shutting down server")
				_ = srv.Shutdown(ctx)
				cancel()
			}()

			addr := ":" + strconv.Itoa(port)
			if err := srv.ListenAndServe(addr); err != nil && ctx.Err() == nil {
				return fmt.Errorf("server: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 8000, "port to listen on")
	cmd.Flags().StringVar(&dataDir, "data-dir", defaultDataDir(), "directory for persistent data")
	cmd.Flags().StringVar(&provider, "provider", "mock", "LLM provider: mock or anthropic")

	return cmd
}

func newIngestCmd() *cobra.Command {
	var patientID int

	cmd := &cobra.Command{
		Use:   "ingest <file>",
		Short: "Ingest a report from file",
		Long:  "Parse and ingest a medical report file through the extraction pipeline. The report is processed, normalized, and stored locally.",
		Example: `  nura ingest --patient-id 1 report.txt
  nura ingest --patient-id 1 /path/to/blood-work.pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			dataDir := cfg.Server.DataDir
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return fmt.Errorf("create data dir: %w", err)
			}

			s, llm, _ := initDeps(dataDir)
			defer s.Close()

			logger.Info("ingesting report", zap.String("file", filePath), zap.Int("patient_id", patientID))

			p := pipeline.NewPipeline(llm, s)
			result, err := p.Run(context.Background(), patientID, string(data), "2024-01-01")
			if err != nil {
				return fmt.Errorf("ingest: %w", err)
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}

	cmd.Flags().IntVar(&patientID, "patient-id", 1, "patient profile ID")

	return cmd
}

func newTraceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trace <id>",
		Short: "Show how to view a trace",
		Long:  "Display instructions for viewing an agent execution trace. Trace viewing requires a running server; this command prints the appropriate curl command.",
		Example: `  nura trace abc123
  nura trace 550e8400-e29b-41d4-a716-446655440000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			traceID := args[0]
			logger.Info("trace lookup", zap.String("trace_id", traceID))
			fmt.Fprintln(os.Stderr, "Trace viewing requires the server to be running.")
			fmt.Fprintf(os.Stderr, "Use: curl http://localhost:%d/agent/debug/trace/%s\n",
				cfg.Server.Port, traceID)
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Display the current version of the nura CLI.",
		Example: `  nura version`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("nura version 0.1.0")
		},
	}
}

func initDeps(dataDir string) (*store.Store, providers.LLMProvider, policy.PolicyEngine) {
	dbPath := filepath.Join(dataDir, cfg.Database.Path)
	s, err := store.New(dbPath)
	if err != nil {
		logger.Fatal("init store", zap.Error(err))
	}

	llm, err := providers.NewProvider(cfg.LLM.Provider, cfg.LLM.APIKey, cfg.LLM.Model)
	if err != nil {
		logger.Fatal("init LLM provider", zap.Error(err))
	}
	logger.Info("LLM provider initialized", zap.String("provider", cfg.LLM.Provider))

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
