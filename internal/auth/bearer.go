package auth

import (
	"fmt"
	"net/http"

	"github.com/clcollins/shim-mcp/internal/config"
)

type bearerProvider struct {
	token  config.CredentialRef
	prefix string
}

func newBearerProvider(cfg config.AuthConfig) *bearerProvider {
	return &bearerProvider{token: cfg.Token, prefix: "Bearer"}
}

func newTokenProvider(cfg config.AuthConfig) *bearerProvider {
	return &bearerProvider{token: cfg.Token, prefix: "token"}
}

func (p *bearerProvider) Name() string {
	if p.prefix == "token" {
		return "token"
	}
	return "bearer"
}

func (p *bearerProvider) Authenticate(req *http.Request) error {
	val, err := p.token.Resolve()
	if err != nil {
		return fmt.Errorf("resolving token: %w", err)
	}

	req.Header.Set("Authorization", p.prefix+" "+val)
	return nil
}
