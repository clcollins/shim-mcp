package auth

import (
	"bytes"
	"fmt"
	"net/http"
	"text/template"

	"github.com/clcollins/shim-mcp/internal/config"
)

type headerProvider struct {
	token      config.CredentialRef
	headerName string
	tmpl       *template.Template
}

func newHeaderProvider(cfg config.AuthConfig) (*headerProvider, error) {
	tmpl, err := template.New("header").Parse(cfg.Template)
	if err != nil {
		return nil, fmt.Errorf("parsing header template: %w", err)
	}

	return &headerProvider{
		token:      cfg.Token,
		headerName: cfg.Header,
		tmpl:       tmpl,
	}, nil
}

func (p *headerProvider) Name() string { return "header" }

func (p *headerProvider) Authenticate(req *http.Request) error {
	val, err := p.token.Resolve()
	if err != nil {
		return fmt.Errorf("resolving token: %w", err)
	}

	data := struct{ Token string }{Token: val}
	var buf bytes.Buffer
	if err := p.tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing header template: %w", err)
	}

	req.Header.Set(p.headerName, buf.String())
	return nil
}
