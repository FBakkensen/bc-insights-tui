# Step 6 — Multi-line KQL Editor Mode (Spec-Driven)

This document defines the precise behavior, contracts, and tests for implementing Step 6 of the chat-first UI: a multi-line KQL editor mode that lets users compose and run multi-line queries directly in the bottom textarea.

## Purpose and scope

- Add an editor mode triggered by the `edit` command that:
  - Increases the chat textarea height and enables newline insertion.
  - Uses Enter to insert a newline while editing.
  - Submits the KQL with Ctrl+Enter.
  - Cancels the edit with Esc and returns to single-line mode without running.
- On submit, reuse Step 5’s execution pipeline (validation, “Running…”, results summary + snapshot, interactive open).

Non-goals (deferred):
- Syntax highlighting, linting, snippets, autocompletion.
- Saved queries management.
- Alternate submit keys or mouse interactions.

## Dependencies and references

- Bubble Tea runtime and key handling; alt screen; focus: https://pkg.go.dev/github.com/charmbracelet/bubbletea
- Bubbles textarea (multi-line input, keymap, focus/blur, width/height): https://pkg.go.dev/github.com/charmbracelet/bubbles/textarea
- Bubbles viewport (scrollback) and table (already used in Step 5).
- Reflow for ANSI-aware wrapping if needed in snapshots.

Portability note on Ctrl+Enter
- Terminals often encode Enter and Ctrl+M similarly (CR). Bubbles’ textarea default InsertNewline binding includes both "enter" and "ctrl+m". We will map submission to a dedicated key binding and dispatch an internal submit message to avoid ambiguity. Tests will simulate the internal submit message to remain deterministic across platforms.

Authoritative docs checked on: Bubble Tea v1.3.x and Bubbles v0.21.x.

## User flow (happy path)

1) In chat mode, user types `edit` and presses Enter.
2) UI switches to editor mode:
   - Textarea height increases (e.g., to 8–10 rows within available space).
   - InsertNewline is enabled; prompt indicates editor state (e.g., `KQL> `) and a helper hint appears: “Ctrl+Enter to run · Esc to cancel”.
   - Focus remains in the textarea.
3) User writes a multi-line KQL (Enter inserts newline) and presses Ctrl+Enter to submit.
4) Program exits editor mode, echoes the submitted KQL (first line + …), appends “Running…” and follows Step 5’s pipeline:
   - Validate → Execute → Append summary + snapshot → Offer “Press Enter to open interactively.”
5) If the user presses Esc while editing, the app exits editor mode, discards the draft (not executed), and appends “Canceled edit.” to the scrollback.

## UI states and focus

- Add mode: `modeKQLEditor`.
- Chat mode: single-line textarea with Enter submit; `edit` transitions to `modeKQLEditor`.
- Editor mode: multi-line textarea with Enter inserting newline; Ctrl+Enter submits; Esc cancels and returns to chat mode.
- Table results mode is unchanged (Step 5).

## Keymap (editor mode)

- Enter → insert newline (textarea default InsertNewline).
- Ctrl+Enter → submit editor buffer (dispatch internal `submitEditorMsg`).
- Esc → cancel editor, return to chat.

Implementation detail: bind a `key.Binding` for submission that recognizes "ctrl+enter" and "ctrl+m" (for terminals that report it), but the actual submission path uses our internal `submitEditorMsg` so tests can bypass terminal differences.

## Contracts (inputs/outputs)

Input
- Command `edit` in chat mode.
- Editor buffer contents (multi-line KQL string).

Pre-flight
- Must be authenticated and have Application Insights App Id set (same checks as Step 5); otherwise show actionable errors on submission.

Validation
- Reuse `appinsights.Client.ValidateQuery(text)`; if empty/whitespace-only → “Query cannot be empty.”

Execution
- Reuse Step 5: timeout via `config.QueryTimeoutSeconds`; call `ExecuteQuery(ctx, query)`.

Output (success)
- Append summary + snapshot; provide “Press Enter to open interactively.”

Output (cancel)
- Append “Canceled edit.” and return to chat mode; no execution.

Logging (per logging guidelines)
- Log entering editor mode and exiting (submit vs cancel).
- On submit, log a stable hash/length of the query (never log full KQL) and timing.
- Log config used (fetch size / timeout) and API status/latency.

## Rendering and layout

- On entering editor mode:
  - Set textarea height to a reasonable default (e.g., 8) but cap to available space after accounting for borders and viewport.
  - Enable InsertNewline: `ta.KeyMap.InsertNewline.SetEnabled(true)`.
  - Set a distinct prompt (e.g., `KQL> `) or dynamic line-number prompt using `SetPromptFunc`.
  - Show a helper hint (small line under the editor) with “Ctrl+Enter to run · Esc to cancel”.
- On WindowSize changes: recompute textarea width/height and viewport height so the editor remains visible and the viewport uses remaining space. Always scroll viewport to bottom on new messages.

## Messages/events (internal)

- Chat `edit` command → transition to `modeKQLEditor`.
- While in editor mode:
  - `tea.KeyMsg{Type: tea.KeyEsc}` → cancel edit.
  - `submitEditorMsg` (from key binding matching Ctrl+Enter) → attempt submit:
    - If buffer trimmed is empty → error message and remain in editor mode.
    - Else leave editor mode, echo, append “Running…”, dispatch KQL command (as in Step 5) → `kqlResultMsg` updates.

## Edge cases and handling

- Empty buffer on submit → “Query cannot be empty.”; stay in editor mode.
- Whitespace-only or comment-only → treat as empty.
- Pasted content with CRLF → normalize to `\n`.
- Very tall queries → editor height caps at max; editing remains smooth; viewport shrinks accordingly.
- Resize while editing → editor resizes; no content loss.
- User types `kql: ...` while in editor → treated as text; only Ctrl+Enter submits.

## Acceptance criteria

- `edit` switches to a multi-line editor with newline insertion, distinct prompt, and visible hint.
- Ctrl+Enter submits the multi-line KQL and triggers the same execution pipeline as Step 5; Esc cancels without running.
- Errors are actionable; logging events are emitted for enter/exit editor and submissions.
- Layout adjusts correctly on resize; viewport remains consistent.

## Test plan (required tests)

Location: `tui/ui_kql_test.go` (or a new `tui/ui_kql_editor_test.go`). Use non-interactive tests with synthetic messages.

Mode transitions
- Enter editor: typing `edit` in chat and pressing Enter switches `mode` to `modeKQLEditor`, enables InsertNewline, increases textarea height, and shows the hint in scrollback.
- Cancel editor: sending `tea.KeyMsg{Type: tea.KeyEsc}` in editor mode returns to `modeChat` and appends “Canceled edit.”

Editing behavior
- Newline insertion: in editor mode, pressing Enter inserts a newline into the textarea value (assert `strings.Contains(ta.Value(), "\n")`).
- Paste handling: simulate `textarea.Paste()` cmd or direct `InsertString` with `\n` and verify content preserved.

Submit behavior
- Empty submit: in editor mode, dispatch `submitEditorMsg` with empty/whitespace buffer → shows “Query cannot be empty.” and stays in editor mode.
- Successful submit: with a fake KQL client and a simple multi-line query, `submitEditorMsg` exits editor mode, appends echo + “Running…”, and posts a `kqlResultMsg` that renders a summary + snapshot + interactive hint.
- Validation error: if `ValidateQuery` returns error, show error and remain or exit per implementation choice (recommended: exit editor mode only after a successful dispatch; otherwise remain and show error).

Resize behavior
- While in `modeKQLEditor`, send `tea.WindowSizeMsg`; verify textarea width/height and viewport height recompute within bounds and no panic occurs.

Integration-style (with fakes)
- Provide a test double for `ValidateQuery`/`ExecuteQuery` as in Step 5 tests. Ensure no network calls.

Notes
- Do not assert exact ANSI; assert presence of hint text, mode transitions, and content mutations.
- For Ctrl+Enter, tests should simulate `submitEditorMsg` directly to avoid terminal differences.

## Implementation checklist (dev tasks)

1) Add `modeKQLEditor` to the model.
2) Handle `edit` command in chat mode:
   - Switch mode, enable InsertNewline, set editor height, set prompt/hint, focus textarea.
3) In editor mode Update:
   - Esc → cancel; append “Canceled edit.”; return to chat.
   - Key binding for Ctrl+Enter → dispatch `submitEditorMsg`.
   - Otherwise forward to `textarea.Update`.
4) On `submitEditorMsg`:
   - Read buffer; trim and validate; empty → error message and stay editing.
   - Else exit editor mode, echo input (truncate for scrollback), append “Running…”, and trigger the Step 5 async KQL command path.
5) On `kqlResultMsg`: unchanged (Step 5 handles results/snapshot/interactive open).
6) Window resize: recompute layout in all modes.
7) Logging: instrument editor enter/exit and submission with context; avoid logging the raw KQL.
8) Tests: add/extend tests as specified; keep non-interactive.

## Windows/terminal notes

- Continue using alt screen and bracketed paste. Keep mouse off by default. On Windows terminals, Ctrl+Enter may not be distinguishable from Enter; the internal `submitEditorMsg` triggered by key bindings ensures consistent behavior. Consider adding a secondary binding (e.g., Alt+Enter) if field testing shows inconsistencies (future enhancement; not required here).
