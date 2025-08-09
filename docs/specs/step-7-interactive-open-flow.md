# Step 7 — Unified Interactive Open Flow (Spec-Driven)

This document defines the behavior and contracts for opening the latest KQL results interactively from either chat mode or editor mode, without the act of opening/closing changing the user's current authoring mode. The selection between chat mode and editor mode remains exclusively a user choice.

## Purpose and scope

- Allow opening the most recent KQL results interactively regardless of whether the user is currently in chat mode or editor mode.
- Opening or closing the interactive table must not implicitly switch between chat and editor modes; only explicit user commands change authoring mode.
- Provide a consistent shortcut to open results interactively from both modes.
- On exit, return to the exact mode (chat or editor) that was active when opening.

Non-goals (deferred):
- New visualization types beyond the existing Bubbles table.
- Persisting a stack of multiple result sets or tabs.
- Mouse interactions or custom sorting/filtering in the table.

## Dependencies and references

- Step 5 (single-line KQL) and Step 6 (multi-line editor) are prerequisites. Their contracts for producing a result snapshot and storing the last results remain unchanged.
- Bubble Tea runtime and Bubbles components (textarea, viewport, table) as in prior steps.
- Logging guidelines in `.github/instructions/logging.instructions.md`.

Related specs:
- Step 5 — Single-line KQL Execute
- Step 6 — Multi-line KQL Editor Mode

## User stories

- As a user in chat mode, after running a query, I can press a shortcut to open the results interactively and, after reviewing, return to chat mode automatically.
- As a user in editor mode, after running a multi-line query, I can press the same shortcut to open the results interactively and, on exit, return to editor mode with my draft intact.
- As a user, I expect that merely opening/closing the interactive view never flips me between chat and editor; only explicit `edit` or `Esc` (to cancel editing) affects authoring mode.

## UX and key bindings

- Shortcut to open interactively (available in both chat and editor):
  - Primary: Enter with empty input (chat) OR a dedicated shortcut that works in both modes.
  - Introduce a universal shortcut: F6 (preferred for consistency across terminals) to "Open last results interactively".
- While in the interactive table view:
  - Esc closes the table and returns to the previously active authoring mode (chat or editor), preserving textarea state and focus semantics.
  - Table retains default navigation bindings (up/down, pgup/pgdn, home/end).

Notes on terminal compatibility
- Enter-with-empty already works in chat mode (Step 5). In editor mode Enter inserts a newline, so Enter-with-empty cannot trigger open there; thus F6 provides a cross-mode trigger.
- Avoid Ctrl+Enter here, as it's already reserved for submitting in the editor and is unreliable across terminals.

## State and mode model

Add a lightweight mechanism to remember the authoring mode at the time of opening:
- Before switching to `modeTableResults`, store `m.returnMode = m.mode` if it is either `modeChat` or `modeKQLEditor`.
- When closing the table (Esc), set `m.mode = m.returnMode` and clear `m.returnMode`.

Constraints
- Do not mutate editor textarea content when opening/closing the interactive table.
- Do not change InsertNewline bindings or prompts when toggling the table; those are driven only by explicit transitions (e.g., `edit`, Esc in editor).

## Contracts (inputs/outputs)

Inputs
- User presses F6 in any authoring mode (chat or editor) or presses Enter with empty input while in chat mode.
- There must be a stored last result set (`haveResults == true`) with columns/rows to render.

Behavior
- If `haveResults` is false, pressing the open shortcut is a no-op and may append a subtle hint: "No results to open." (optional).
- If `haveResults` is true, initialize the interactive table from stored `lastColumns/lastRows/lastTable` and switch to `modeTableResults`.
- On Esc inside the table, switch back to the prior authoring mode stored in `returnMode` and do not alter textarea contents or keymaps.

Outputs
- Visual: table view replaces the top panel while open; on close, original panel returns exactly as it was.
- Scrollback: optional message when opening/closing (e.g., "Opened results table." / "Closed results table.")

## Logging

- Log the user action when opening interactively: `action=open_interactive` with `rowCount`, `columnCount`, and `tableName` (no data values).
- Log closing action with the restored mode.
- Do not log full result content or sensitive values.

## Edge cases

- No last results: pressing F6 should do nothing or append a gentle hint; must not error.
- Extremely wide tables: rely on existing sizing logic; no change required.
- Resize while open: table resizes as today; on close, prior layout persists.
- Editor with large draft: ensure draft content and height are untouched after closing the table.

## Acceptance criteria

- From chat mode: Enter on empty input or F6 opens the interactive table; Esc returns to chat. Textarea state before opening equals state after closing.
- From editor mode: F6 opens the interactive table; Esc returns to editor mode with the exact draft preserved and InsertNewline still enabled.
- Opening and closing do not implicitly toggle between chat/editor; only `edit` and editor-Esc (cancel) change authoring mode.
- Logging captures open/close events with non-sensitive metadata.

## Test plan (required tests)

Location: `tui/ui_kql_test.go` and/or `tui/ui_kql_editor_test.go`.

Chat mode tests
- With `haveResults=true` and empty textarea value, sending `enter` switches to `modeTableResults`; sending Esc then restores `modeChat`.
- With `haveResults=true`, simulate F6 key in chat mode → `modeTableResults`; Esc restores `modeChat`.
- With `haveResults=false`, F6 is a no-op or appends a hint; remains in `modeChat`.

Editor mode tests
- Entering editor (`edit`) then setting `haveResults=true`, simulate F6 → switches to `modeTableResults` while preserving editor buffer and InsertNewline enabled.
- Esc inside table restores `modeKQLEditor`; verify textarea value and keymap unchanged.

Cross-cutting tests
- After closing table from either mode, pressing Ctrl+Enter in editor still submits; pressing Enter in chat still submits/open-last per existing rules.
- Resize during table view and after close does not panic and retains consistent dimensions.

## Implementation checklist (for a later PR; do not implement now)

1) Add field `returnMode` to model to remember the authoring mode during table view.
2) Add a global key handler for F6 in both chat and editor modes that triggers the "open last results" action when `haveResults` is true.
3) When opening: set `returnMode` to current authoring mode; initialize table; `mode = modeTableResults`.
4) When closing (Esc in table): set `mode = returnMode`; clear `returnMode`; optionally append a message.
5) Ensure no mutation of textarea content, prompts, or keymaps during the open/close cycle.
6) Add logs for open/close with metadata (row/column counts, table name, restored mode).
7) Add/extend tests per this spec for chat, editor, and cross-cutting scenarios.

## Notes

- Keep the code changes minimal and in line with idiomatic Bubble Tea. Avoid adding extra modes or state beyond `returnMode`.
- Prefer a single shortcut (F6) for universal open. If future field testing shows terminal conflicts, consider an alternate like Alt+o.
