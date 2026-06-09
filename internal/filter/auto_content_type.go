package filter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type AutoContentType struct{}

func (f *AutoContentType) Name() string { return "auto_content_type" }

func (f *AutoContentType) FilterRequest(ctx Context, req *http.Request) (*http.Request, error) {
	if req.Body == nil {
		return req, nil
	}

	if req.Header.Get("Content-Type") != "" {
		return req, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return req, nil
	}

	if len(body) == 0 {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return req, nil
	}

	if json.Valid(body) {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return req, nil
}
