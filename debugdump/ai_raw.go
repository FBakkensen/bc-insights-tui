package debugdump

// Lightweight helper to write Application Insights raw request/response debug captures

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AIRawHeaders represents selected HTTP headers with redactions applied.
type AIRawHeaders map[string]string

// AIRawRequest captures request details
type AIRawRequest struct {
	StartedAt string       `yaml:"startedAt"`
	Method    string       `yaml:"method"`
	URL       string       `yaml:"url"`
	Headers   AIRawHeaders `yaml:"headers"`
	Body      string       `yaml:"body"`
	BodyBytes int          `yaml:"bodyBytes"`
	Truncated bool         `yaml:"truncated"`
}

// AIRawResponse captures response details
type AIRawResponse struct {
	CompletedAt string       `yaml:"completedAt"`
	Status      int          `yaml:"status"`
	DurationMs  int64        `yaml:"durationMs"`
	Headers     AIRawHeaders `yaml:"headers"`
	Body        string       `yaml:"body"`
	BodyBytes   int          `yaml:"bodyBytes"`
	Truncated   bool         `yaml:"truncated"`
}

// AIRawError represents an error payload
type AIRawError struct {
	Message string `yaml:"message"`
}

// AIRawCapture is the root document for request-only capture
type AIRawCapture struct {
	Version    int            `yaml:"version"`
	CapturedAt string         `yaml:"capturedAt"`
	Request    AIRawRequest   `yaml:"request"`
	Response   *AIRawResponse `yaml:"response,omitempty"`
	Error      *AIRawError    `yaml:"error"`
}

// AIRawFullCapture is identical shape; kept for semantic clarity if needed later.
type AIRawFullCapture = AIRawCapture

// RedactHeaders returns a copy of selected headers with secrets redacted.
func RedactHeaders(in map[string]string) AIRawHeaders {
	out := make(AIRawHeaders, len(in))
	for k, v := range in {
		lk := strings.ToLower(strings.TrimSpace(k))
		switch lk {
		case "authorization":
			out[k] = "Bearer <redacted>"
		case "cookie", "set-cookie", "x-api-key", "api-key", "ocp-apim-subscription-key":
			out[k] = "<redacted>"
		default:
			out[k] = v
		}
	}
	return out
}

// TruncateBody enforces a max length; returns body string to write, original size, and truncation flag.
// maxBytes == 0 means unlimited.
func TruncateBody(b []byte, maxBytes int) (string, int, bool) {
	if b == nil {
		return "", 0, false
	}
	orig := len(b)
	if maxBytes <= 0 || orig <= maxBytes {
		return string(b), orig, false
	}
	// Keep first maxBytes; indicate truncation.
	return string(b[:maxBytes]), orig, true
}

// FormatBodyPrettyJSON attempts to pretty-print JSON; if parsing fails, falls back to raw string.
// Returns the string to write (possibly truncated), the original network payload size in bytes, and whether truncation occurred.
func FormatBodyPrettyJSON(b []byte, maxBytes int) (string, int, bool) {
	if b == nil {
		return "", 0, false
	}
	origLen := len(b)
	// Try to pretty print
	var any interface{}
	var pretty []byte
	if err := json.Unmarshal(b, &any); err == nil {
		pb, perr := json.MarshalIndent(any, "", "  ")
		if perr == nil {
			pretty = pb
		}
	}
	if len(pretty) == 0 {
		// Fallback to original
		if maxBytes <= 0 || origLen <= maxBytes {
			return string(b), origLen, false
		}
		return string(b[:maxBytes]), origLen, true
	}
	// Apply truncation to pretty output, but keep BodyBytes as original network size
	if maxBytes <= 0 || len(pretty) <= maxBytes {
		return string(pretty), origLen, false
	}
	return string(pretty[:maxBytes]), origLen, true
}

// WriteAIRawRequest writes the request-only capture atomically.
func WriteAIRawRequest(path string, req AIRawCapture) error {
	return writeYAMLAtomic(path, req)
}

// WriteAIRawFull writes the full capture atomically.
func WriteAIRawFull(path string, full AIRawFullCapture) error {
	return writeYAMLAtomic(path, full)
}

// ResolvePath ensures a sane default location and extension for the raw file.
// If path is empty, use logs/appinsights-raw.yaml. If relative, ensure under logs/ by default.
func ResolvePath(in string) (string, error) {
	p := strings.TrimSpace(in)
	if p == "" {
		p = filepath.Join("logs", "appinsights-raw.yaml")
	}
	// If path has no dir and isn't under logs, put it under logs by default
	if !filepath.IsAbs(p) {
		dir := filepath.Dir(p)
		if dir == "." {
			p = filepath.Join("logs", p)
		}
	}
	// If no extension, default to .yaml. If extension is present (even if non-yaml), keep as provided.
	if ext := strings.ToLower(filepath.Ext(p)); ext == "" {
		p = p + ".yaml"
	}
	// Normalize to OS-specific separators and clean up any ./ segments.
	p = filepath.Clean(filepath.FromSlash(p))
	return p, nil
}

func writeYAMLAtomic(path string, doc any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("empty path for AI raw capture")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create logs dir: %w", err)
	}
	// Create temp file in same dir
	tmp, err := os.CreateTemp(dir, "ai-raw-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	enc := yaml.NewEncoder(tmp)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("failed to encode yaml: %w", err)
	}
	if err := enc.Close(); err != nil { // flush encoder
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("failed to close yaml encoder: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	// Atomic replace
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("failed to move temp into place: %w", err)
	}
	return nil
}

// Now returns UTC RFC3339Nano time string for timestamps
func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
