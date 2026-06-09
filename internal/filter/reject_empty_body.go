package filter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type RejectEmptyBody struct{}

func (f *RejectEmptyBody) Name() string { return "reject_empty_body" }

func (f *RejectEmptyBody) FilterRequest(ctx Context, req *http.Request) (*http.Request, error) {
	method := strings.ToUpper(req.Method)
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return req, nil
	}

	if req.Body == nil {
		return nil, fmt.Errorf("%s request requires a body", method)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, fmt.Errorf("%s request requires a non-empty body", method)
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return req, nil
}
