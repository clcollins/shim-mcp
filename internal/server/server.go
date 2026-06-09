package server

import (
	"github.com/clcollins/shim-mcp/internal/config"
	"github.com/clcollins/shim-mcp/internal/proxy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func New(cfg *config.Config, p *proxy.Proxy) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "shim-mcp",
		Version: "dev",
	}, &mcp.ServerOptions{
		Instructions: "HTTP proxy server for authenticated API requests. " +
			"Use 'list_services' to discover available services, " +
			"then 'http_request' to make requests.",
	})

	registerTools(server, p, cfg)

	return server
}
