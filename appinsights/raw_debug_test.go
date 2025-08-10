package appinsights

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type fakeRoundTripper struct {
	resp *http.Response
	err  error
}

func (f *fakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f.resp, f.err }

func installFakeTransport(c *Client, rt http.RoundTripper) { c.httpClient.Transport = rt }

func readYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	return m
}

func TestAIRawCapture_Success(t *testing.T) {
	t.Setenv("TEST_MODE", "1")
	dir := t.TempDir()
	out := filepath.Join(dir, "cap.yaml")
	t.Setenv("BCINSIGHTS_AI_RAW_ENABLE", "true")
	t.Setenv("BCINSIGHTS_AI_RAW_FILE", out)
	t.Setenv("BCINSIGHTS_AI_RAW_MAX_BYTES", "1024")

	tok := &oauth2.Token{AccessToken: "t", Expiry: time.Now().Add(time.Hour)}
	c := NewClient(tok, "appId")
	body := `{"tables": [{"name": "PrimaryResult", "columns": [], "rows": []}]}`
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}, "x-ms-request-id": {"rid"}, "x-ms-correlation-request-id": {"cid"}}, Body: io.NopCloser(strings.NewReader(body))}
	installFakeTransport(c, &fakeRoundTripper{resp: resp})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.ExecuteQuery(ctx, "traces | limit 1"); err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}

	st, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Size() == 0 {
		t.Fatalf("expected non-empty capture file")
	}

	m := readYAMLMap(t, out)
	if _, ok := m["request"]; !ok {
		t.Fatalf("missing request section")
	}
	respm := m["response"].(map[string]any)
	// YAML numbers may decode as int/float; tolerate both
	switch v := respm["status"].(type) {
	case int:
		if v != 200 {
			t.Fatalf("expected status 200")
		}
	case int64:
		if v != 200 {
			t.Fatalf("expected status 200")
		}
	case float64:
		if int(v) != 200 {
			t.Fatalf("expected status 200")
		}
	default:
		t.Fatalf("unexpected status type %T", v)
	}
}

func TestAIRawCapture_TransportError(t *testing.T) {
	t.Setenv("TEST_MODE", "1")
	dir := t.TempDir()
	out := filepath.Join(dir, "cap.yaml")
	t.Setenv("BCINSIGHTS_AI_RAW_ENABLE", "true")
	t.Setenv("BCINSIGHTS_AI_RAW_FILE", out)

	tok := &oauth2.Token{AccessToken: "t", Expiry: time.Now().Add(time.Hour)}
	c := NewClient(tok, "appId")
	installFakeTransport(c, &fakeRoundTripper{resp: nil, err: fmt.Errorf("boom")})

	if _, err := c.ExecuteQuery(context.Background(), "traces | limit 1"); err == nil {
		t.Fatalf("expected error")
	}
	m := readYAMLMap(t, out)
	if _, ok := m["error"]; !ok {
		t.Fatalf("missing error")
	}
}

func TestAIRawCapture_Truncation(t *testing.T) {
	t.Setenv("TEST_MODE", "1")
	dir := t.TempDir()
	out := filepath.Join(dir, "cap.yaml")
	t.Setenv("BCINSIGHTS_AI_RAW_ENABLE", "true")
	t.Setenv("BCINSIGHTS_AI_RAW_FILE", out)
	t.Setenv("BCINSIGHTS_AI_RAW_MAX_BYTES", "10") // tiny

	tok := &oauth2.Token{AccessToken: "t", Expiry: time.Now().Add(time.Hour)}
	c := NewClient(tok, "appId")
	// 30 bytes body to force truncation
	long := strings.Repeat("x", 30)
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(long))}
	installFakeTransport(c, &fakeRoundTripper{resp: resp})

	if _, err := c.ExecuteQuery(context.Background(), "traces | limit 1"); err == nil {
		t.Fatalf("expected parse error for invalid JSON body")
	}
	// Use non-200 to still write capture with body present
	resp = &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(long))}
	installFakeTransport(c, &fakeRoundTripper{resp: resp})
	_, _ = c.ExecuteQuery(context.Background(), "traces | limit 1")

	m := readYAMLMap(t, out)
	respm := m["response"].(map[string]any)
	if tr, ok := respm["truncated"].(bool); !ok || !tr {
		t.Fatalf("expected response.truncated true, got %v", respm["truncated"])
	}
}

func TestAIRawCapture_Disabled_NoFile(t *testing.T) {
	t.Setenv("TEST_MODE", "1")
	dir := t.TempDir()
	out := filepath.Join(dir, "cap.yaml")
	// Ensure disabled
	t.Setenv("BCINSIGHTS_AI_RAW_ENABLE", "false")
	t.Setenv("BCINSIGHTS_AI_RAW_FILE", out)

	tok := &oauth2.Token{AccessToken: "t", Expiry: time.Now().Add(time.Hour)}
	c := NewClient(tok, "appId")
	body := `{"tables": [{"name": "PrimaryResult", "columns": [], "rows": []}]}`
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	installFakeTransport(c, &fakeRoundTripper{resp: resp})
	_, _ = c.ExecuteQuery(context.Background(), "traces | limit 1")

	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("expected no capture file when disabled, err=%v", err)
	}
}

type seqRoundTripper struct {
	resps []*http.Response
	idx   int
}

func (s *seqRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if s.idx >= len(s.resps) {
		return nil, fmt.Errorf("no more")
	}
	resp := s.resps[s.idx]
	s.idx++
	return resp, nil
}

func TestAIRawCapture_Overwrite(t *testing.T) {
	t.Setenv("TEST_MODE", "1")
	dir := t.TempDir()
	out := filepath.Join(dir, "cap.yaml")
	t.Setenv("BCINSIGHTS_AI_RAW_ENABLE", "true")
	t.Setenv("BCINSIGHTS_AI_RAW_FILE", out)

	tok := &oauth2.Token{AccessToken: "t", Expiry: time.Now().Add(time.Hour)}
	c := NewClient(tok, "appId")
	body1 := `{"tables": [{"name": "PrimaryResult", "columns": [], "rows": [[1]]}]}`
	body2 := `{"tables": [{"name": "PrimaryResult", "columns": [], "rows": [[1],[2],[3]]}]}`
	srt := &seqRoundTripper{resps: []*http.Response{
		{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body1))},
		{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body2))},
	}}
	installFakeTransport(c, srt)

	_, _ = c.ExecuteQuery(context.Background(), "traces | limit 1")
	_, _ = c.ExecuteQuery(context.Background(), "traces | limit 1")

	// File should reflect the last capture (rows length 3 imputed by bytes length)
	st, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Size() == 0 {
		t.Fatalf("expected non-empty file")
	}
}

// Use standard library io.NopCloser (Go >= 1.20)
