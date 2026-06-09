package auth

import "net/http"

type noneProvider struct{}

func (p *noneProvider) Name() string                       { return "none" }
func (p *noneProvider) Authenticate(_ *http.Request) error { return nil }
