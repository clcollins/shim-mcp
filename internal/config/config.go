package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var validAuthTypes = map[string]bool{
	"none":   true,
	"basic":  true,
	"bearer": true,
	"token":  true,
	"header": true,
}

type Config struct {
	Services map[string]ServiceConfig `mapstructure:"services"`
}

type ServiceConfig struct {
	BaseURL       string            `mapstructure:"base_url"`
	Auth          AuthConfig        `mapstructure:"auth"`
	Headers       map[string]string `mapstructure:"headers"`
	Filters       FilterConfig      `mapstructure:"filters"`
	TLSSkipVerify bool              `mapstructure:"tls_skip_verify"`
}

type FilterConfig struct {
	Request  RequestFilterConfig  `mapstructure:"request"`
	Response ResponseFilterConfig `mapstructure:"response"`
}

type RequestFilterConfig struct {
	ValidateJSONBody bool `mapstructure:"validate_json_body"`
	AutoContentType  bool `mapstructure:"auto_content_type"`
	RejectEmptyBody  bool `mapstructure:"reject_empty_body"`
}

type ResponseFilterConfig struct {
	StripFields []string `mapstructure:"strip_fields"`
}

type AuthConfig struct {
	Type     string        `mapstructure:"type"`
	Username CredentialRef `mapstructure:"username"`
	Token    CredentialRef `mapstructure:"token"`
	Header   string        `mapstructure:"header"`
	Template string        `mapstructure:"template"`
}

type CredentialRef struct {
	File   string `mapstructure:"file"`
	Env    string `mapstructure:"env"`
	Format string `mapstructure:"format"`
	Key    string `mapstructure:"key"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("config must define at least one service")
	}

	for name, svc := range cfg.Services {
		if err := validateService(name, &svc); err != nil {
			return nil, err
		}
		cfg.Services[name] = svc
	}

	return &cfg, nil
}

func validateService(name string, svc *ServiceConfig) error {
	if svc.BaseURL == "" {
		return fmt.Errorf("service %q: base_url is required", name)
	}

	svc.BaseURL = strings.TrimRight(svc.BaseURL, "/")

	u, err := url.Parse(svc.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("service %q: invalid base_url %q", name, svc.BaseURL)
	}

	if svc.Auth.Type == "" {
		return fmt.Errorf("service %q: auth.type is required", name)
	}

	if !validAuthTypes[svc.Auth.Type] {
		return fmt.Errorf("service %q: unknown auth type %q (valid: basic, bearer, token, header)", name, svc.Auth.Type)
	}

	if err := validateAuth(name, &svc.Auth); err != nil {
		return err
	}

	return nil
}

func validateAuth(name string, auth *AuthConfig) error {
	switch auth.Type {
	case "none":
		// No credentials needed
	case "basic":
		if err := validateCredentialRef(name, "username", &auth.Username); err != nil {
			return err
		}
		if err := validateCredentialRef(name, "token", &auth.Token); err != nil {
			return err
		}
	case "bearer", "token":
		if err := validateCredentialRef(name, "token", &auth.Token); err != nil {
			return err
		}
	case "header":
		if auth.Header == "" {
			return fmt.Errorf("service %q: auth.header is required for header auth", name)
		}
		if auth.Template == "" {
			return fmt.Errorf("service %q: auth.template is required for header auth", name)
		}
		if err := validateCredentialRef(name, "token", &auth.Token); err != nil {
			return err
		}
	}

	return nil
}

func validateCredentialRef(serviceName, field string, ref *CredentialRef) error {
	if err := ref.Validate(); err != nil {
		return fmt.Errorf("service %q: auth.%s: %w", serviceName, field, err)
	}
	if err := ref.expandAndValidatePath(); err != nil {
		return fmt.Errorf("service %q: auth.%s: %w", serviceName, field, err)
	}
	return nil
}

// Validate checks that a CredentialRef has exactly one source mode.
func (ref *CredentialRef) Validate() error {
	hasFile := ref.File != ""
	hasEnv := ref.Env != ""

	if hasFile && hasEnv {
		return fmt.Errorf("cannot set both file and env")
	}
	if !hasFile && !hasEnv {
		return fmt.Errorf("must set either file or env")
	}
	if !hasFile && ref.Key != "" {
		return fmt.Errorf("key requires file (not env)")
	}
	if !hasFile && ref.Format != "" {
		return fmt.Errorf("format requires file (not env)")
	}
	return nil
}

// Resolve reads the credential value from the configured source.
func (ref *CredentialRef) Resolve() (string, error) {
	if ref.Env != "" {
		val := os.Getenv(ref.Env)
		if val == "" {
			return "", fmt.Errorf("environment variable %q is not set or empty", ref.Env)
		}
		return val, nil
	}

	data, err := os.ReadFile(ref.File)
	if err != nil {
		return "", fmt.Errorf("reading credential file: %w", err)
	}

	format := inferFormat(ref.File, ref.Format)

	if ref.Key == "" && format == "text" {
		val := strings.TrimSpace(string(data))
		if val == "" {
			return "", fmt.Errorf("credential file is empty: %s", ref.File)
		}
		return val, nil
	}

	if ref.Key == "" {
		val := strings.TrimSpace(string(data))
		if val == "" {
			return "", fmt.Errorf("credential file is empty: %s", ref.File)
		}
		return val, nil
	}

	if format == "env" {
		return resolveEnvFile(string(data), ref.Key)
	}

	var obj any
	switch format {
	case "json":
		if err := json.Unmarshal(data, &obj); err != nil {
			return "", fmt.Errorf("parsing JSON: %w", err)
		}
	case "yaml":
		if err := yaml.Unmarshal(data, &obj); err != nil {
			return "", fmt.Errorf("parsing YAML: %w", err)
		}
		obj = normalizeYAML(obj)
	default:
		return "", fmt.Errorf("key extraction requires format json, yaml, or env, got %q", format)
	}

	return extractPath(obj, ref.Key)
}

func (ref *CredentialRef) expandAndValidatePath() error {
	if ref.File == "" {
		return nil
	}

	if strings.Contains(ref.File, "..") {
		return fmt.Errorf("path traversal not allowed: %q", ref.File)
	}

	if strings.HasPrefix(ref.File, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("expanding ~: %w", err)
		}
		ref.File = filepath.Join(home, ref.File[2:])
	}

	ref.File = filepath.Clean(ref.File)
	return nil
}

func inferFormat(filePath, explicit string) string {
	if explicit != "" {
		return explicit
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".env":
		return "env"
	default:
		return "text"
	}
}

type pathSegment struct {
	Key   string
	Index int
	IsIdx bool
}

func parsePath(path string) ([]pathSegment, error) {
	if !strings.HasPrefix(path, ".") {
		return nil, fmt.Errorf("path must start with '.': %q", path)
	}

	rest := path[1:]
	var segs []pathSegment

	for rest != "" {
		if strings.HasPrefix(rest, "[") {
			end := strings.Index(rest, "]")
			if end < 0 {
				return nil, fmt.Errorf("unterminated array index in path: %q", path)
			}
			idxStr := rest[1:end]
			idx, err := strconv.Atoi(idxStr)
			if err != nil || idx < 0 {
				return nil, fmt.Errorf("invalid array index %q in path: %q", idxStr, path)
			}
			segs = append(segs, pathSegment{Index: idx, IsIdx: true})
			rest = strings.TrimPrefix(rest[end+1:], ".")
		} else if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				return nil, fmt.Errorf("unterminated quoted key in path: %q", path)
			}
			segs = append(segs, pathSegment{Key: rest[1 : end+1]})
			rest = strings.TrimPrefix(rest[end+2:], ".")
		} else {
			endIdx := len(rest)
			for i, c := range rest {
				if c == '.' || c == '[' || c == '"' {
					endIdx = i
					break
				}
			}
			if endIdx == 0 {
				return nil, fmt.Errorf("empty key segment in path: %q", path)
			}
			segs = append(segs, pathSegment{Key: rest[:endIdx]})
			rest = strings.TrimPrefix(rest[endIdx:], ".")
		}
	}

	if len(segs) == 0 {
		return nil, fmt.Errorf("empty path: %q", path)
	}

	return segs, nil
}

func extractPath(obj any, path string) (string, error) {
	segs, err := parsePath(path)
	if err != nil {
		return "", err
	}

	current := obj
	for _, seg := range segs {
		if seg.IsIdx {
			arr, ok := current.([]any)
			if !ok {
				return "", fmt.Errorf("expected array at index [%d], got %T", seg.Index, current)
			}
			if seg.Index >= len(arr) {
				return "", fmt.Errorf("array index [%d] out of bounds (length %d)", seg.Index, len(arr))
			}
			current = arr[seg.Index]
		} else {
			m, ok := current.(map[string]any)
			if !ok {
				return "", fmt.Errorf("expected object at key %q, got %T", seg.Key, current)
			}
			val, ok := m[seg.Key]
			if !ok {
				return "", fmt.Errorf("key %q not found", seg.Key)
			}
			current = val
		}
	}

	str, ok := current.(string)
	if !ok {
		return "", fmt.Errorf("value at path %q is not a string, got %T", path, current)
	}
	return str, nil
}

// normalizeYAML converts yaml.Unmarshal output (map[any]any) to map[string]any
// for consistent path traversal with JSON.
func normalizeYAML(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = normalizeYAML(v)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[fmt.Sprint(k)] = normalizeYAML(v)
		}
		return result
	case []any:
		for i, item := range val {
			val[i] = normalizeYAML(item)
		}
		return val
	default:
		return v
	}
}

func resolveEnvFile(content, key string) (string, error) {
	prefix := key + "="
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			val := line[len(prefix):]
			val = strings.TrimSpace(val)
			if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
				(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
				val = val[1 : len(val)-1]
			}
			if val == "" {
				return "", fmt.Errorf("key %q has empty value in env file", key)
			}
			return val, nil
		}
	}
	return "", fmt.Errorf("key %q not found in env file", key)
}
