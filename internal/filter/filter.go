package filter

import "net/http"

type Context struct {
	ServiceName string
	Method      string
	Path        string
}

type RequestFilter interface {
	Name() string
	FilterRequest(ctx Context, req *http.Request) (*http.Request, error)
}

type ResponseFilter interface {
	Name() string
	FilterResponse(ctx Context, resp *http.Response) (*http.Response, error)
}
