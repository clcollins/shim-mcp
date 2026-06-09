package filter

import "net/http"

type RequestFilter interface {
	Name() string
	FilterRequest(req *http.Request) (*http.Request, error)
}

type ResponseFilter interface {
	Name() string
	FilterResponse(resp *http.Response) (*http.Response, error)
}
