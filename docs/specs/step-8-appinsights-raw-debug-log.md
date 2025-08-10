# Step 8 — App Insights Raw Request/Response Debug File (Spec-Driven)

This document defines a targeted debugging feature to capture the most raw details of each Application Insights query request and its response into a dedicated YAML file that is recreated for every new request. The goal is to make troubleshooting easy without bloating daily logs.

## Purpose and scope

- When enabled, capture the raw HTTP request (method, URL, headers with redactions, and JSON body) and the raw HTTP response (status, headers, and JSON body) for the last executed Application Insights KQL call.
- Write this to a single dedicated YAML file that is overwritten at the start/end of each request, ensuring the file never grows unbounded.
- Keep normal logs concise; use this file only for deep dive diagnostics.

Non-goals (deferred):
- Persisting historical captures or multiple files per request.
- A UI viewer inside the TUI.
- Capturing non-App-Insights traffic.

## Dependencies and references

- Code path: `appinsights/client.go` → `ExecuteQuery(ctx, kql)` creates the HTTP request, sets headers, executes, reads body, and parses JSON.
- Logging guidelines: `.github/instructions/logging.instructions.md` (log setting changes, user actions, errors). This feature complements, not replaces, the daily log.
- Config system: `config/` module for env/flags/file precedence; settings naming should be consistent with existing keys and env prefixes.
- Azure Monitor Log Analytics response format (tables/columns/rows): https://learn.microsoft.com/azure/azure-monitor/logs/api/response-format

## Requirements (contract)

Functional
- A dedicated raw debug file is produced for Application Insights queries when the feature is enabled.
- The file is recreated (overwritten atomically) for every new request. It always reflects the most recent KQL call.
- Request section includes: timestamp, method, URL (including path and query params), selected headers (with Authorization redacted), request body (JSON string rendered as a YAML scalar) and byte length.
- Response section includes: timestamp, duration, HTTP status code, selected headers (notably `x-ms-request-id`, `x-ms-correlation-request-id`, `content-type`), response body (JSON string rendered as a YAML scalar) and byte length.
- On error (HTTP error or transport/timeout), include an `error` object with message and, if available, response body and status.
- The Authorization header value MUST be redacted; no tokens or keys should be written verbatim.
- The KQL query text MAY be included in the request body (it’s part of the JSON payload). That’s acceptable for local debugging; warn in docs that it can contain sensitive details.

Performance and lifecycle
- Disabled by default. When disabled, there is zero file I/O for this feature.
- When enabled, use an atomic write (temp file + rename) to avoid partially written files.
- The file should live under the existing `logs/` directory by default.

Concurrency and determinism
- If multiple KQL requests run concurrently, last writer wins. The file reflects the most recently completed write. Avoid complex locking; keep implementation simple.
- Each request should first write a “request started” capture (request-only), then rewrite the file with both request and response once the call completes. This guarantees a useful artifact even if the process crashes mid-call.

Security and privacy
- Always redact `Authorization` and any known API key headers. Don’t log cookies.
- Do not log secrets to the daily log; only to this dedicated file with redactions applied.
- Provide an optional size cap to guard against very large bodies; if truncation occurs, mark it with a `truncated` flag and record original sizes.

## Config and UX

New settings (file + env only; no new CLI flags required now):
- Setting name: `debug.appInsightsRawEnable` (bool)
  - Default: `false`
  - Env: `BCINSIGHTS_AI_RAW_ENABLE=true|false`
  - Behavior: When true, the feature is active.
- Setting name: `debug.appInsightsRawFile` (string)
  - Default: `logs/appinsights-raw.json` (resolved relative to the app’s working directory; create `logs/` if missing)
  - Env: `BCINSIGHTS_AI_RAW_FILE=custom\path\file.json`
- Setting name: `debug.appInsightsRawMaxBytes` (int; optional)
  - Default: `1048576` (1 MiB) per body (request and response). `0` means unlimited.
  - Env: `BCINSIGHTS_AI_RAW_MAX_BYTES=2097152`

UX notes
- When the feature toggles (on/off) or path changes, log an info line in the daily log with old→new values per logging guidelines.
- On every capture, log a single concise info line to the daily log: `ai_raw_dump_written` with duration, status, and path.

## File format and schema

Write a single YAML document with the following shape (keys stable for tests):

```
version: 1
capturedAt: "2025-08-10T12:34:56.789Z"
request:
  startedAt: "..."
  method: POST
  url: "https://api.applicationinsights.io/v1/apps/<appId>/query"
  headers:
    content-type: application/json
    authorization: "Bearer <redacted>"
  body: "{\"query\":\"traces | take 10\"}"
  bodyBytes: 42
  truncated: false
response:
  completedAt: "..."
  status: 200
  durationMs: 1234
  headers:
    x-ms-request-id: "..."
    x-ms-correlation-request-id: "..."
    content-type: application/json
  body: "{\"tables\":[...]}"
  bodyBytes: 2048
  truncated: false
error: null
```

Notes
- If the response is an error, set `response.status` accordingly and populate `error` with `{ "message": "..." }`. If no response body read is possible, omit `response` and keep the error.
- If a size cap triggers, set `truncated: true` and keep `bodyBytes` as the original size before truncation.

## Implementation plan

Where to hook
- Implement inside `appinsights.Client.ExecuteQuery`. This method has everything needed: URL, headers, body JSON, and the response.

Helper component
- Add a tiny internal helper (package `logging` or a new package `debugdump`) with two functions:
  - `WriteAIRawRequest(path string, req AIRawCapture) error` — serialize to YAML and write atomically.
  - `WriteAIRawFull(path string, full AIRawFullCapture) error` — overwrite with the full request+response payload in YAML.

Data structs (internal)
- `AIRawCapture` with fields for request-only.
- `AIRawFullCapture` embedding the request capture and adding response and error fields.

Execution flow
1) If `debug.appInsightsRawEnable == false`, do nothing (fast path).
2) Build the request body JSON (already done today). Before performing `Do(req)`, construct the request capture with redacted headers and possibly truncated body based on `debug.appInsightsRawMaxBytes`.
3) Resolve the target path. If relative, prefix with `logs/` and default extension `.yaml`. Ensure directory exists.
4) Write the request-only capture using `WriteAIRawRequest` (atomic). Also log one concise info line to the daily log with the path.
5) Execute the HTTP request. Read the response bytes (already done); compute duration, extract headers (`x-ms-request-id`, `x-ms-correlation-request-id`, `content-type`). Apply truncation if needed.
6) Build the full capture (request + response + error if any) and call `WriteAIRawFull` to overwrite atomically.
7) Parsing continues as normal; errors are returned unchanged.

Redaction rules
- Replace `Authorization` header with `Bearer <redacted>` if present.
- If an instrumentation key or API key header variant is ever used, replace its value with `<redacted>` as well.

Concurrency note
- This feature intentionally maintains only a single file. Multiple overlapping requests may interleave; the last writer wins. That’s acceptable for a lightweight debugging tool. We’ll document this limitation.

## Logging integration (daily log)

- On first enable/disable and on path/max-bytes changes, log: setting old→new.
- On each capture start/end:
  - Start: `ai_raw_dump_started` with `path` and `body_bytes`.
  - End: `ai_raw_dump_written` with `path`, `status`, `duration_ms`, and `resp_bytes`.
- Never include the raw bodies in the daily log.

## Acceptance criteria

- With `BCINSIGHTS_AI_RAW_ENABLE=true`, running any KQL (Step 5/6 flows) produces a `logs/appinsights-raw.yaml` file that contains the request and response sections, redacts Authorization, and shows accurate durations and sizes.
- The file is overwritten for each new KQL call (no unbounded growth). Writes are atomic (no partial JSON observed in manual tailing).
- When disabled, no file is created or updated.
- If a timeout or HTTP error occurs, the file still contains the request section and an error with status/body when available.
- Truncation works when `BCINSIGHTS_AI_RAW_MAX_BYTES` is set to a small value; `truncated` is true and `bodyBytes` reflects the original size.

## Test plan (required tests)

Location: `appinsights/client_test.go` (extend) or a new `appinsights/azure_client_test.go` section.

Unit-style with fakes
- Injection: Replace `c.httpClient.Transport` with a fake `RoundTripper` to control responses; point `debug.appInsightsRawFile` to a temp dir.
- Enabled path (200):
  - Assert file exists and is valid YAML with `version`, `request`, and `response`.
  - Assert `authorization` is redacted; `status` is 200; `durationMs` > 0.
- Error path (non-200):
  - Response 401/429 with JSON body → assert `response.status` and `error.message` present and body captured.
- Transport error/timeout:
  - Simulate timeout → file has request-only + `error` and no response.
- Truncation:
  - Set `maxBytes=32` and return a larger body → assert `truncated: true` and `bodyBytes` equals original size.
- Disabled path:
  - With `enable=false`, assert no file produced.
- Overwrite semantics:
  - Perform two sequential calls → second file contents differ and reflect the last call only.

Notes
- Keep tests non-interactive; do not run the TUI. Use temporary directories for file paths.
- Avoid leaking secrets in assertions; ensure redaction is verified explicitly.

## Implementation checklist (dev tasks)

1) Config: add fields and env bindings for `debug.appInsightsRawEnable`, `debug.appInsightsRawFile`, `debug.appInsightsRawMaxBytes`; persist and log changes per guidelines.
2) Helper: implement atomic JSON writers (`WriteAIRawRequest`/`WriteAIRawFull`) with directory creation and redaction helpers.
3) Client: integrate calls in `ExecuteQuery` around the HTTP `Do` call.
4) Logging: add concise info logs for start/end events (no raw bodies).
5) Tests: add/extend tests as above with a fake `RoundTripper` and temp file path.
6) Docs: add a short note to `README.md` troubleshooting (optional) warning that the raw file may contain sensitive KQL; keep it disabled by default.

## Windows/terminal notes

- Paths: default `logs/appinsights-raw.json`. On Windows, ensure `logs\` exists and use `os.CreateTemp` + `os.Rename` for atomicity. File encoding is UTF-8 with `\r\n` newlines allowed in string values.
- No terminal interaction required; this feature operates entirely in the background during request execution.
