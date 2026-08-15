# Replace webview with a Gio config window

Date: 2026-08-16
Status: Approved for planning

## Summary

Replace the `webview_go` config window with a native window built with Gio
(`gioui.org`). The window keeps the same job: show the version, pick a capture
audio device, and set a pinned volume. Gio needs no cgo on Windows, so this also
removes the last cgo dependency and lets the build drop the C toolchain and the
WebView2 runtime.

## Context

- The whole app is Windows-only: WASAPI audio (`go-wca`), Windows services, the
  event log, and many syscalls. `main` stops early when `GOOS != "windows"`. A
  cross-platform GUI does not make the app portable; Gio is chosen because it is
  native and cgo-free, not for portability.
- `webview_go` is the only cgo dependency. CI installs MinGW (for cgo) and the
  WebView2 runtime only because of it.
- The current UI (`ui.go` + `templates/index.html`) is a 480x320 window: a
  version label, an audio input device `<select>`, and a volume slider (0-100).
  A background goroutine re-applies the volume every 100 ms to pin it.

## Goals

- Build the same window with Gio.
- Remove `webview_go`, `templates/index.html`, and the `//go:embed`.
- Build with `CGO_ENABLED=0`; drop the MinGW and WebView2 steps from CI.
- Keep the volume pinning behavior (continuous re-apply).
- Keep `audio.go` and the OLE/COM initialization unchanged.

## Non-goals

- No change to audio logic, services, updater, or logging.
- No true cross-platform support; the app stays Windows-only.
- No Thai text. The two Thai labels become English, so Gio's built-in Go fonts
  render them without an extra font asset.

## Target design

### Widgets

- Version: a `material.Label`.
- Device picker: Gio has no ComboBox. Use a scrollable selectable list
  (`layout.List` of clickable rows, the selected row highlighted) instead of a
  dropdown. Same result, less code, no popup overlay.
- Volume: a `material.Slider` (0-100%) with a live `%` readout.
- Error line: a small inline label for the last error, in place of the old JS
  `alert()`.

Labels: "Audio input device" and "Volume".

### State

A small controller holds the selection and is shared between the UI and the
pinning goroutine:

```go
type audioControl struct {
    mu         sync.Mutex
    endpointID string
    volume     float32 // 0..1
}
```

- `func (c *audioControl) setEndpoint(id string)`
- `func (c *audioControl) setVolume(v float32)` // 0..1
- `func (c *audioControl) snapshot() (id string, v float32)`

The pinning goroutine keeps its current shape: every 100 ms it reads the
snapshot and, when the endpoint is set and the volume is in range, calls
`setAudioVolume`. On error it logs with `slog`.

### Event and thread model

- `ui()` creates the `app.Window`, starts the Gio frame loop in a goroutine, and
  calls `app.Main()` on the main goroutine (Gio requires `app.Main` there).
- `ui()` is still the last call in `main`, so `app.Main` runs on the main
  goroutine, after OLE `CoInitializeEx`.
- The pinning goroutine and the audio COM calls keep their current threading.

### Data flow

1. On start the UI calls `getAudioInputDevices`.
2. It selects the default endpoint (`IsDefaultAudioEndpoint`) and calls
   `setEndpoint` with it; the volume starts at 1.0 (100%).
3. Clicking a device row calls `setEndpoint`. Moving the slider calls
   `setVolume`.
4. The pinning goroutine applies the current endpoint and volume every 100 ms.

### Error handling

- Device enumeration failure: show the error in the inline label and render an
  empty list; log with `slog`.
- Volume apply failure: log with `slog`; the next tick retries (the pinning loop
  already retries by design).

## Dependencies

- Add `gioui.org` (and its transitive dependencies) with `go get`.
- Remove `github.com/webview/webview_go`.
- `go mod tidy` to update `go.mod` and `go.sum`.

## CI changes (`.github/workflows/golang.yml`)

- Remove the "Install WebView2 Runtime" step.
- Remove the "Install MinGW-w64" step.
- Build with `CGO_ENABLED=0`. Keep `-ldflags "-H windowsgui ..."` and the
  `goversioninfo` step.

## Testing

- `go build` and `go vet` for `GOOS=windows` in CI (now cgo-free, so the runner
  no longer needs a C compiler).
- `audioControl` gets one small `assert`-based self-check for `setVolume`
  clamping and `snapshot` returning the last set values. No UI test framework.
- Manual check on Windows: the window opens, lists devices, selects the default,
  the slider sets and pins the volume.

## Risks

- Gio's first frame needs a valid GPU path on Windows; the CI build only
  compiles, it does not run the window, so this is a runtime, not a build, risk.
- Gio has no ComboBox; the selectable list is a deliberate substitute for the
  dropdown and changes the look, not the function.
- Device friendly names with non-Latin glyphs may not render with the Go fonts.
  Out of scope for now; device names are usually Latin.

## Migration steps (high level)

1. Add `gioui.org`; remove `webview_go`.
2. Rewrite `ui.go` with Gio; add `audioControl`.
3. Delete `templates/index.html` and the embed.
4. Update the CI workflow (drop MinGW and WebView2; `CGO_ENABLED=0`).
5. `go mod tidy`, build for `GOOS=windows`, run the self-check.
6. Update `CONTEXT.md`; commit.
