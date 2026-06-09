package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/clcollins/shim-mcp/internal/config"
	"github.com/clcollins/shim-mcp/internal/proxy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type httpRequestInput struct {
	Service     string            `json:"service" jsonschema:"Name of the configured service to send the request to"`
	Method      string            `json:"method" jsonschema:"HTTP method (GET POST PUT DELETE PATCH HEAD)"`
	Path        string            `json:"path,omitempty" jsonschema:"URL path to append to the service base URL"`
	Headers     map[string]string `json:"headers,omitempty" jsonschema:"Additional HTTP headers to include"`
	QueryParams map[string]string `json:"query_params,omitempty" jsonschema:"Query parameters to append to the URL"`
	Body        string            `json:"body,omitempty" jsonschema:"Request body (typically JSON)"`
}

type httpResponseOutput struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

type listServicesInput struct{}

type listServicesOutput struct {
	Services []serviceInfo `json:"services"`
}

type serviceInfo struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

func registerTools(server *mcp.Server, p *proxy.Proxy, cfg *config.Config) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "http_request",
		Description: "Make an authenticated HTTP request to a configured service",
	}, httpRequestHandler(p))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_services",
		Description: "List all configured services and their base URLs",
	}, listServicesHandler(cfg))
}

func httpRequestHandler(p *proxy.Proxy) mcp.ToolHandlerFor[httpRequestInput, httpResponseOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input httpRequestInput) (*mcp.CallToolResult, httpResponseOutput, error) {
		if input.Service == "" {
			return nil, httpResponseOutput{}, fmt.Errorf("service is required")
		}
		if input.Method == "" {
			return nil, httpResponseOutput{}, fmt.Errorf("method is required")
		}

		proxyReq := &proxy.Request{
			Service:     input.Service,
			Method:      input.Method,
			Path:        input.Path,
			Headers:     input.Headers,
			QueryParams: input.QueryParams,
			Body:        input.Body,
		}

		resp, err := p.Do(ctx, proxyReq)
		if err != nil {
			return nil, httpResponseOutput{}, fmt.Errorf("request failed: %w", err)
		}

		return nil, httpResponseOutput{
			StatusCode: resp.StatusCode,
			Headers:    resp.Headers,
			Body:       resp.Body,
		}, nil
	}
}

func listServicesHandler(cfg *config.Config) mcp.ToolHandlerFor[listServicesInput, listServicesOutput] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input listServicesInput) (*mcp.CallToolResult, listServicesOutput, error) {
		services := make([]serviceInfo, 0, len(cfg.Services))
		for name, svc := range cfg.Services {
			services = append(services, serviceInfo{
				Name:    name,
				BaseURL: svc.BaseURL,
			})
		}

		var sb strings.Builder
		for _, s := range services {
			fmt.Fprintf(&sb, "- %s: %s\n", s.Name, s.BaseURL)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: sb.String()},
			},
		}, listServicesOutput{Services: services}, nil
	}
}
