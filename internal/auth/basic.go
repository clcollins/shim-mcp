package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/clcollins/shim-mcp/internal/config"
)

type basicProvider struct {
	username config.CredentialRef
	token    config.CredentialRef
}

func newBasicProvider(cfg config.AuthConfig) *basicProvider {
	return &basicProvider{
		username: cfg.Username,
		token:    cfg.Token,
	}
}

func (p *basicProvider) Name() string { return "basic" }

func (p *basicProvider) Authenticate(req *http.Request) error {
	user, err := p.username.Resolve()
	if err != nil {
		return fmt.Errorf("resolving username: %w", err)
	}

	token, err := p.token.Resolve()
	if err != nil {
		return fmt.Errorf("resolving token: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + token))
	req.Header.Set("Authorization", "Basic "+encoded)
	return nil
}
