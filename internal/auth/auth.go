package auth

import (
	"fmt"
	"net/http"

	"github.com/clcollins/shim-mcp/internal/config"
)

type AuthProvider interface {
	Name() string
	Authenticate(req *http.Request) error
}

func NewAuthProvider(cfg config.AuthConfig) (AuthProvider, error) {
	switch cfg.Type {
	case "none":
		return &noneProvider{}, nil
	case "basic":
		return newBasicProvider(cfg), nil
	case "bearer":
		return newBearerProvider(cfg), nil
	case "token":
		return newTokenProvider(cfg), nil
	case "header":
		return newHeaderProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown auth type: %q", cfg.Type)
	}
}
