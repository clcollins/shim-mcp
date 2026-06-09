package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/clcollins/shim-mcp/internal/config"
	"github.com/clcollins/shim-mcp/internal/proxy"
	"github.com/clcollins/shim-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server on stdio",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	configPath := cfgFile
	if configPath == "" {
		return fmt.Errorf("--config is required")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("config loaded", "services", len(cfg.Services), "config", configPath)

	p, err := proxy.New(cfg)
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
