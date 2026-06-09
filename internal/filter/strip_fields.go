package filter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type StripFields struct {
	fields map[string]bool
}

func NewStripFields(fields []string) *StripFields {
	m := make(map[string]bool, len(fields))
	for _, f := range fields {
		m[f] = true
	}
	return &StripFields{fields: m}
}

func (f *StripFields) Name() string { return "strip_fields" }

func (f *StripFields) FilterResponse(ctx Context, resp *http.Response) (*http.Response, error) {
	if len(f.fields) == 0 {
		return resp, nil
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		return resp, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil
	}
	_ = resp.Body.Close()

	if len(body) == 0 {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	f.stripRecursive(data)

	stripped, err := json.Marshal(data)
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	resp.Body = io.NopCloser(bytes.NewReader(stripped))
	resp.ContentLength = int64(len(stripped))
	return resp, nil
}

func (f *StripFields) stripRecursive(data any) {
	switch v := data.(type) {
	case map[string]any:
		for key := range v {
			if f.fields[key] {
				delete(v, key)
			} else {
				f.stripRecursive(v[key])
			}
		}
	case []any:
		for _, item := range v {
			f.stripRecursive(item)
		}
	}
}
