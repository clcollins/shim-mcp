package filter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ValidateJSONBody struct{}

func (f *ValidateJSONBody) Name() string { return "validate_json_body" }

func (f *ValidateJSONBody) FilterRequest(ctx Context, req *http.Request) (*http.Request, error) {
	if req.Body == nil {
		return req, nil
	}

	method := strings.ToUpper(req.Method)
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return req, nil
	}

	ct := req.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		return req, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}

	if len(body) == 0 {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return req, nil
	}

	if !json.Valid(body) {
		var syntaxErr *json.SyntaxError
		decoder := json.NewDecoder(bytes.NewReader(body))
		var raw json.RawMessage
		decErr := decoder.Decode(&raw)
		if decErr != nil {
			if errors.As(decErr, &syntaxErr) {
				return nil, fmt.Errorf("invalid JSON body at byte offset %d: %w", syntaxErr.Offset, decErr)
			}
			return nil, fmt.Errorf("invalid JSON body: %w", decErr)
		}
		return nil, fmt.Errorf("invalid JSON body")
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return req, nil
}
