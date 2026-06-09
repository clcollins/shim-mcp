package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

var sensitiveHeaders = map[string]bool{
	"Authorization":       true,
	"X-Api-Key":           true,
	"X-Auth-Token":        true,
	"Proxy-Authorization": true,
}

func ScrubHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	if h == nil {
		return result
	}

	for key, vals := range h {
		if sensitiveHeaders[key] {
			continue
		}
		if len(vals) > 0 {
			result[key] = vals[0]
		}
	}
	return result
}

func ScrubError(err error, credentials []string) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	for _, cred := range credentials {
		if cred != "" {
			msg = strings.ReplaceAll(msg, cred, "[REDACTED]")
		}
	}
	return fmt.Errorf("%s", msg)
}
