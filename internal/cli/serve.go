package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/clcollins/shim-mcp/internal/config"
	"github.com/clcollins/shim-mcp/internal/logging"
	"github.com/clcollins/shim-mcp/internal/proxy"
	"github.com/clcollins/shim-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var logLevel string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server on stdio",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVar(&logLevel, "log-level", "", "log level: debug, info, warn, error (default: error)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	levelStr := logLevel
	if levelStr == "" {
		levelStr = os.Getenv("SHIM_MCP_LOG_LEVEL")
	}
	if levelStr == "" {
		levelStr = "error"
	}

	parsedLevel, err := logging.ParseLevel(levelStr)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	var levelVar slog.LevelVar
	levelVar.Set(parsedLevel)

	logger := logging.NewLogger(&levelVar)

	configPath := cfgFile
	if configPath == "" {
		return fmt.Errorf("--config is required")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.LogLevel != "" && logLevel == "" && os.Getenv("SHIM_MCP_LOG_LEVEL") == "" {
		configLevel, parseErr := logging.ParseLevel(cfg.LogLevel)
		if parseErr != nil {
			return fmt.Errorf("invalid config log_level: %w", parseErr)
		}
		levelVar.Set(configLevel)
	}

	logger.Info("config loaded", "services", len(cfg.Services), "config", configPath)

	p, err := proxy.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("creating proxy: %w", err)
	}

	srv := server.New(cfg, p)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("starting MCP server on stdio")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
