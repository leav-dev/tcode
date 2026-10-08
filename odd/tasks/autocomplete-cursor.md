# Autocomplete cursor API (tcode editor)

## Goal
Expose cursor position to Lua so `tcode.autocomplete` (buffer-word completion, `alt+space`) can complete the prefix under the cursor.

## Contract (must match extension)
- Lua: `tcode.cursor()` returns `line, col` 1-indexed numbers (matching `tcode.line(n)` 1-indexed convention).
- `line:sub(1, col-1)` = bytes before cursor on current line (ByteCol semantics, ASCII-safe for `[A-Za-z0-9_]` prefix).
- No buffer: return nil (0 values), like `tcode.buffer()` no-buffer path. Extension `pcall(tcode.cursor)` handles nil.
- Missing function on old editor: extension degrades with English message (already implemented).

## Surfaces
- `internal/view/editor_view.go`: add public `CursorPosition() (line, byteCol int)` returning 0-indexed line + byte col.
- `internal/controller/app.go`: implement `Cursor() (line, col int, ok bool)` for ScriptAPI (0-indexed Go side).
- `internal/ext/script.go`: extend `ScriptAPI` interface + `tcode.cursor` binding (translate Go 0-indexed to Lua 1-indexed).
- Tests: `internal/ext/script_test.go` (fakeAPI + cursor binding tests), `internal/ext/manager_test.go` (forwardAPI stub), `internal/controller/app_ext_test.go` or focused integration test.
- Docs: `docs/extension-system.md` Lua API table row + short note.

## Acceptance
- `go test ./internal/ext/ ./internal/view/ ./internal/controller/` green (or scoped equivalents).
- Harness in `../tcode-extention/autocomplete/harness` still green (contract unchanged).
- No behavior change to existing `tcode.*` functions.

## Evidence
- Commit tagged `v1.0.4` — feat(scripting): expose cursor position as tcode.cursor (8 files, +136).
- Tests green: internal/ext, internal/view, internal/controller. Vet clean.
