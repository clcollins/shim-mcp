package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/clcollins/shim-mcp/internal/auth"
	"github.com/clcollins/shim-mcp/internal/config"
	"github.com/clcollins/shim-mcp/internal/filter"
)

var validMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"PATCH":  true,
	"DELETE": true,
	"HEAD":   true,
}

type authProvider interface {
	Name() string
	Authenticate(req *http.Request) error
}

type Request struct {
	Service     string
	Method      string
	Path        string
	Headers     map[string]string
	QueryParams map[string]string
	Body        string
}

type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

type Proxy struct {
	services        map[string]config.ServiceConfig
	auth            map[string]authProvider
	clients         map[string]*http.Client
	maxResponseSize int64
	requestFilters  map[string][]filter.RequestFilter
	responseFilters map[string][]filter.ResponseFilter
	logger          *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) (*Proxy, error) {
	authProviders := make(map[string]authProvider, len(cfg.Services))
	httpClients := make(map[string]*http.Client, len(cfg.Services))
	reqFilters := make(map[string][]filter.RequestFilter, len(cfg.Services))
	respFilters := make(map[string][]filter.ResponseFilter, len(cfg.Services))

	defaultClient := &http.Client{Timeout: 30 * time.Second}

	for name, svc := range cfg.Services {
		provider, err := auth.NewAuthProvider(svc.Auth)
		if err != nil {
			return nil, fmt.Errorf("creating auth provider for %q: %w", name, err)
		}
		authProviders[name] = provider

		if svc.TLSSkipVerify {
			httpClients[name] = &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // per-service opt-in
				},
			}
		} else {
			httpClients[name] = defaultClient
		}

		rf, rsf := buildFilters(svc.Filters)
		if len(rf) > 0 {
			reqFilters[name] = rf
		}
		if len(rsf) > 0 {
			respFilters[name] = rsf
		}
	}

	return &Proxy{
		services:        cfg.Services,
		auth:            authProviders,
		clients:         httpClients,
		maxResponseSize: 10 * 1024 * 1024,
		requestFilters:  reqFilters,
		responseFilters: respFilters,
		logger:          logger,
	}, nil
}

func buildFilters(cfg config.FilterConfig) ([]filter.RequestFilter, []filter.ResponseFilter) {
	var req []filter.RequestFilter
	var resp []filter.ResponseFilter

	if cfg.Request.AutoContentType {
		req = append(req, &filter.AutoContentType{})
	}
	if cfg.Request.RejectEmptyBody {
		req = append(req, &filter.RejectEmptyBody{})
	}
	if cfg.Request.ValidateJSONBody {
		req = append(req, &filter.ValidateJSONBody{})
	}

	if len(cfg.Response.StripFields) > 0 {
		resp = append(resp, filter.NewStripFields(cfg.Response.StripFields))
	}

	return req, resp
}

func (p *Proxy) Do(ctx context.Context, proxyReq *Request) (*Response, error) {
	start := time.Now()

	svc, ok := p.services[proxyReq.Service]
	if !ok {
		return nil, fmt.Errorf("unknown service: %q", proxyReq.Service)
	}

	method := strings.ToUpper(proxyReq.Method)
	if !validMethods[method] {
		return nil, fmt.Errorf("invalid HTTP method: %q", proxyReq.Method)
	}

	fullURL, err := buildURL(svc.BaseURL, proxyReq.Path, proxyReq.QueryParams)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(fullURL, svc.BaseURL) {
		return nil, fmt.Errorf("URL %q does not match service base URL %q", fullURL, svc.BaseURL)
	}

	var body io.Reader
	if proxyReq.Body != "" {
		body = strings.NewReader(proxyReq.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	for key, val := range svc.Headers {
		req.Header.Set(key, val)
	}
	for key, val := range proxyReq.Headers {
		req.Header.Set(key, val)
	}

	provider, ok := p.auth[proxyReq.Service]
	if !ok {
		return nil, fmt.Errorf("no auth provider for service %q", proxyReq.Service)
	}
	if err := provider.Authenticate(req); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	filterCtx := filter.Context{
		ServiceName: proxyReq.Service,
		Method:      method,
		Path:        proxyReq.Path,
	}

	for _, f := range p.requestFilters[proxyReq.Service] {
		req, err = f.FilterRequest(filterCtx, req)
		if err != nil {
			return nil, fmt.Errorf("request filter %s: %w", f.Name(), err)
		}
	}

	client := p.clients[proxyReq.Service]
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	for _, f := range p.responseFilters[proxyReq.Service] {
		filtered, filterErr := f.FilterResponse(filterCtx, resp)
		if filterErr != nil {
			return nil, fmt.Errorf("response filter %s: %w", f.Name(), filterErr)
		}
		resp = filtered
	}

	limitedReader := io.LimitReader(resp.Body, p.maxResponseSize+1)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	bodyStr := string(respBody)
	if int64(len(respBody)) > p.maxResponseSize {
		bodyStr = bodyStr[:p.maxResponseSize]
	}

	p.logger.Info("request completed",
		"service", proxyReq.Service,
		"method", method,
		"path", proxyReq.Path,
		"status", resp.StatusCode,
		"response_bytes", len(respBody),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    ScrubHeaders(resp.Header),
		Body:       bodyStr,
	}, nil
}

func (p *Proxy) Services() map[string]config.ServiceConfig {
	return p.services
}

func buildURL(baseURL, path string, queryParams map[string]string) (string, error) {
	if path == "" {
		if len(queryParams) == 0 {
			return baseURL, nil
		}
		return baseURL + "?" + encodeParams(queryParams), nil
	}

	if strings.Contains(path, "://") {
		return "", fmt.Errorf("path must not contain a scheme — use a relative path: %q", path)
	}

	path = "/" + strings.TrimLeft(path, "/")
	result := strings.TrimRight(baseURL, "/") + path

	if len(queryParams) > 0 {
		result += "?" + encodeParams(queryParams)
	}

	return result, nil
}

func encodeParams(params map[string]string) string {
	parts := make([]string, 0, len(params))
	for k, v := range params {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return strings.Join(parts, "&")
}
