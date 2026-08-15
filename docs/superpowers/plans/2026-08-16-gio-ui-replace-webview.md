# Gio UI Replacing Webview — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `webview_go` config window with a native Gio window, and remove the last cgo dependency.

**Architecture:** Move the volume/endpoint selection state into a small, OS-independent `internal/audioctl` package that is unit-testable anywhere. Rewrite `ui.go` (package `main`) to render the same surface (version label, device picker, volume slider) with Gio. Keep `audio.go`, the OLE/COM init, and the 100 ms volume-pinning goroutine behavior.

**Tech Stack:** Go 1.24.3, `gioui.org` (Gio, cgo-free on Windows), `log/slog`, `github.com/moutend/go-wca` (unchanged).

**Spec:** `docs/superpowers/specs/2026-08-16-gio-ui-replace-webview-design.md`

## Global Constraints

- Module path: `github.com/willywotz/fivem`. Go directive: `go 1.24.3`.
- The app is Windows-only. Build target for all build/vet checks: `GOOS=windows GOARCH=amd64`.
- No cgo: build with `CGO_ENABLED=0`. Keep the linker flags `-H windowsgui -X 'main.version=...' -X 'main.BaseURL=...'`.
- Logging is `log/slog` only (no `fmt.Print`, no stdlib `log`).
- UI labels are English: "Audio input device", "Volume".
- Keep the continuous volume pinning: a goroutine re-applies the volume every 100 ms.
- Naming: American English.
- This machine is Linux. Windows binaries cannot be executed here. "Build/vet" checks run as `GOOS=windows CGO_ENABLED=0`; they type-check the whole package but do not run it. Package `main` cannot be `go test`ed on Linux (it imports Windows-only packages); only `internal/audioctl` runs locally. Runtime verification of the window happens on Windows CI or a Windows machine.

---

### Task 1: `internal/audioctl` state package

**Files:**
- Create: `internal/audioctl/control.go`
- Test: `internal/audioctl/control_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func New() *Control` — returns a Control with volume 1.0 (100%) and empty endpoint.
  - `func (c *Control) SetEndpoint(id string)` — set the selected endpoint id.
  - `func (c *Control) SetVolume(v float32)` — set volume, clamped to [0, 1].
  - `func (c *Control) Snapshot() (endpointID string, volume float32)` — read both under the lock.

- [ ] **Step 1: Write the failing test**

Create `internal/audioctl/control_test.go`:

```go
package audioctl_test

import (
	"testing"

	"github.com/willywotz/fivem/internal/audioctl"
)

func TestControlDefaults(t *testing.T) {
	c := audioctl.New()
	id, v := c.Snapshot()
	if id != "" || v != 1 {
		t.Fatalf("defaults: got (%q, %v), want (\"\", 1)", id, v)
	}
}

func TestControlSetAndSnapshot(t *testing.T) {
	c := audioctl.New()
	c.SetEndpoint("abc")
	c.SetVolume(0.5)
	id, v := c.Snapshot()
	if id != "abc" || v != 0.5 {
		t.Fatalf("set: got (%q, %v), want (\"abc\", 0.5)", id, v)
	}
}

func TestControlVolumeClamp(t *testing.T) {
	c := audioctl.New()
	c.SetVolume(2)
	if _, v := c.Snapshot(); v != 1 {
		t.Fatalf("clamp high: got %v, want 1", v)
	}
	c.SetVolume(-1)
	if _, v := c.Snapshot(); v != 0 {
		t.Fatalf("clamp low: got %v, want 0", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audioctl/`
Expected: FAIL — package/`audioctl.New` does not exist (compile error).

- [ ] **Step 3: Write minimal implementation**

Create `internal/audioctl/control.go`:

```go
// Package audioctl holds the audio endpoint and volume selection shared
// between the UI and the goroutine that pins the volume.
package audioctl

import "sync"

type Control struct {
	mu         sync.Mutex
	endpointID string
	volume     float32 // 0..1
}

func New() *Control {
	return &Control{volume: 1}
}

func (c *Control) SetEndpoint(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.endpointID = id
}

func (c *Control) SetVolume(v float32) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.volume = v
}

func (c *Control) Snapshot() (endpointID string, volume float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.endpointID, c.volume
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/audioctl/`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/audioctl/
git commit -m "feat: add audioctl.Control for endpoint and volume state"
```

---

### Task 2: Rewrite `ui.go` with Gio and drop webview

**Files:**
- Modify: `go.mod`, `go.sum` (add `gioui.org`, remove `github.com/webview/webview_go`)
- Rewrite: `ui.go`
- Delete: `templates/index.html` (and the `templates/` directory)
- Modify: `CONTEXT.md` (document the new UI)

**Interfaces:**
- Consumes: `audioctl.New/SetEndpoint/SetVolume/Snapshot` (Task 1); `getAudioInputDevices() ([]AudioDevice, error)` and `setAudioVolume(endpointId string, volumeLevel float32) error` and the `AudioDevice` struct (all existing in `audio.go`); `version` (package `main`).
- Produces: `func ui()` (called from `main`, unchanged call site); `func pinVolume(c *audioctl.Control)`; `func runUI(w *app.Window, devices []AudioDevice, c *audioctl.Control, loadErr error) error`.

- [ ] **Step 1: Add Gio, remove webview**

```bash
go get gioui.org@latest
go mod edit -droprequire github.com/webview/webview_go
```

Record the resolved Gio version printed by `go get` (used in the next step).

- [ ] **Step 2: Confirm the Gio API for the resolved version**

Gio's window/event API changed across v0.x releases. Before writing `ui.go`, confirm the exact identifiers for the resolved version using the context7 MCP tools: `resolve-library-id` for "gioui.org", then `query-docs` for the window loop (`app.Window`, `Event`/`Events`, `FrameEvent`, `DestroyEvent`, `app.NewContext`/`layout.NewContext`), `material.NewTheme` + shaper setup, `material.Slider`, and `widget.Clickable.Clicked`. The reference implementation in Step 3 targets the v0.6+ API (`new(app.Window)`, `w.Event()`, `app.NewContext`, `app.FrameEvent`, `app.DestroyEvent`). If the resolved version differs, adjust the identifiers accordingly; the structure stays the same.

- [ ] **Step 3: Rewrite `ui.go`**

Replace the entire contents of `ui.go` with:

```go
package main

import (
	"image/color"
	"log/slog"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/willywotz/fivem/internal/audioctl"
)

func ui() {
	control := audioctl.New()

	// Enumerate devices on this goroutine, where COM was initialized in main.
	devices, err := getAudioInputDevices()
	if err != nil {
		slog.Error("error getting audio input devices", "err", err)
	}
	for _, d := range devices {
		if d.IsDefaultAudioEndpoint {
			control.SetEndpoint(d.ID)
		}
	}

	go pinVolume(control)

	go func() {
		w := new(app.Window)
		w.Option(app.Title("fivem tools"), app.Size(unit.Dp(480), unit.Dp(320)))
		if err := runUI(w, devices, control, err); err != nil {
			slog.Error("ui loop failed", "err", err)
		}
		os.Exit(0)
	}()

	app.Main()
}

// pinVolume re-applies the selected volume to the selected endpoint every
// 100 ms so the level stays pinned against outside changes.
func pinVolume(c *audioctl.Control) {
	for range time.Tick(100 * time.Millisecond) {
		id, v := c.Snapshot()
		if id == "" || v < 0 || v > 1 {
			continue
		}
		if err := setAudioVolume(id, v); err != nil {
			slog.Error("error setting volume", "err", err)
		}
	}
}

func runUI(w *app.Window, devices []AudioDevice, c *audioctl.Control, loadErr error) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var deviceList widget.List
	deviceList.Axis = layout.Vertical

	rows := make([]widget.Clickable, len(devices))
	selected := -1
	for i, d := range devices {
		if d.IsDefaultAudioEndpoint {
			selected = i
		}
	}

	var vol widget.Float
	vol.Value = 1

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			for i := range rows {
				if rows[i].Clicked(gtx) {
					selected = i
					c.SetEndpoint(devices[i].ID)
				}
			}
			c.SetVolume(vol.Value)

			layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
					// Header: title + version
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
							layout.Rigid(material.H6(th, "fivem tools").Layout),
							layout.Rigid(material.Body2(th, version).Layout),
						)
					}),
					// Error line (only when device load failed)
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if loadErr == nil {
							return layout.Dimensions{}
						}
						l := material.Body2(th, "Error: "+loadErr.Error())
						l.Color = color.NRGBA{R: 0xB0, A: 0xFF}
						return l.Layout(gtx)
					}),
					// Device picker
					layout.Rigid(material.Body1(th, "Audio input device").Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.List(th, &deviceList).Layout(gtx, len(devices),
							func(gtx layout.Context, i int) layout.Dimensions {
								label := material.Body1(th, devices[i].Name)
								if i == selected {
									label.Font.Weight = font.Bold
								}
								return material.Clickable(gtx, &rows[i], label.Layout)
							})
					}),
					// Volume
					layout.Rigid(material.Body1(th, "Volume: "+itoa(int(vol.Value*100))+"%").Layout),
					layout.Rigid(material.Slider(th, &vol).Layout),
				)
			})

			e.Frame(gtx.Ops)
		}
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
```

Note the imports this reference needs but the block above may not have listed if you adjust it: `gioui.org/font` (for `font.Bold`) and `strconv` (for `itoa`). Add them to the import group. Keep imports sorted by path. Remove `itoa` and import `strconv` directly at the call site if you prefer; either is fine as long as it compiles.

- [ ] **Step 4: Delete the old template**

```bash
git rm templates/index.html
```

(There are no other files under `templates/`; the directory goes away with the file.)

- [ ] **Step 5: Tidy modules**

```bash
go mod tidy
```

Expected: `github.com/webview/webview_go` disappears from `go.mod`; `gioui.org` and its dependencies appear.

- [ ] **Step 6: Verify the Windows build type-checks, cgo-free**

Run: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`
Expected: builds with no errors. (This is now a real check — with webview gone there is no cgo blocker.)

Run: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./`
Expected: no errors.

Run: `gofmt -l ui.go internal/audioctl/`
Expected: no output.

If the build reports unused or missing imports (e.g. `font`, `strconv`, `image/color`), fix the import group and re-run until clean.

- [ ] **Step 7: Update `CONTEXT.md`**

Replace the webview description with the Gio one. Add under a "UI" section:

```markdown
## UI

`ui()` builds the config window with Gio (`gioui.org`), not webview. It shows
the version, a scrollable list of capture audio devices, and a volume slider
(0-100%). Picking a device or moving the slider updates `audioctl.Control`.
`pinVolume` re-applies the selected volume to the selected endpoint every
100 ms. Gio needs no cgo on Windows, so the build has no cgo dependency and
needs no WebView2 runtime.
```

- [ ] **Step 8: Commit**

```bash
git add ui.go go.mod go.sum CONTEXT.md
git rm -r --cached templates 2>/dev/null || true
git commit -m "feat: replace webview config window with Gio"
```

---

### Task 3: Drop the C toolchain from CI

**Files:**
- Modify: `.github/workflows/golang.yml`

**Interfaces:**
- Consumes: nothing from earlier tasks at runtime; depends on Task 2 having removed the cgo dependency.
- Produces: a CI job that builds cgo-free and runs the `audioctl` test.

- [ ] **Step 1: Remove the WebView2 and MinGW steps**

Delete these two steps from `.github/workflows/golang.yml` (the "Install WebView2 Runtime" step and the "Install MinGW-w64" step):

```yaml
      - name: Install WebView2 Runtime
        run: |
          Invoke-WebRequest -Uri "https://go.microsoft.com/fwlink/p/?LinkId=2124703" -OutFile "MicrosoftEdgeWebview2Setup.exe"
          Start-Process .\MicrosoftEdgeWebview2Setup.exe -ArgumentList "/silent","/install" -Wait
        shell: pwsh

      - name: Install MinGW-w64
        run: choco install mingw -y
        shell: pwsh
```

- [ ] **Step 2: Build cgo-free and add a test step**

Change the "build go binary" step to set `CGO_ENABLED=0`, and add a test step before it. The build command keeps the same ldflags. Replace the build step with:

```yaml
      - name: test
        run: go test ./internal/...
        shell: pwsh

      - name: build go binary
        env:
          CGO_ENABLED: '0'
        run: |
          go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@53cb51b8aa6b6b62ab8196e66a766ea7598c67fa -64 -file-version '${{ github.ref_name }}' -product-version '${{ github.ref_name }}'
          go build -ldflags="-s -w -H windowsgui -X 'main.version=${{ github.ref_name }}' -X 'main.BaseURL=${{ vars.FIVEM_BASE_URL }}'" -o fivem-windows-amd64.exe .
        shell: pwsh
```

- [ ] **Step 3: Sanity-check the workflow YAML**

Run: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o /tmp/fivem-check.exe .`
Expected: an `.exe` is produced with no cgo and no C compiler. This mirrors what CI now does.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/golang.yml
git commit -m "ci: drop MinGW and WebView2, build cgo-free"
```

---

## Self-Review

**Spec coverage:**
- Rewrite `ui.go` with Gio → Task 2.
- Remove `webview_go`, `templates/index.html`, the embed → Task 2.
- `CGO_ENABLED=0`, drop MinGW + WebView2 from CI → Task 3.
- Keep volume pinning → Task 2 (`pinVolume`, 100 ms).
- Keep `audio.go` and OLE/COM init unchanged → Task 2 touches neither.
- English labels, no Thai, no font asset → Task 2 Step 3.
- `audioControl` self-check → Task 1 (as `internal/audioctl`, runnable on Linux).
- Widgets: label, selectable list, slider, inline error → Task 2 Step 3.
- Event/thread model: frame loop goroutine + `app.Main()` on main → Task 2 Step 3.

**Placeholder scan:** No TBDs. The one deliberate open item — the exact Gio identifiers — is bounded by a concrete verification step (Task 2 Step 2) against the resolved version, with a working reference targeting the v0.6+ API, not a "figure it out later".

**Type consistency:** `audioctl.Control` methods (`New`, `SetEndpoint`, `SetVolume`, `Snapshot`) are named the same in Task 1 and Task 2. `pinVolume(*audioctl.Control)` and `runUI(*app.Window, []AudioDevice, *audioctl.Control, error)` match their call sites in `ui()`. `AudioDevice` fields used (`ID`, `Name`, `IsDefaultAudioEndpoint`) match `audio.go`.

**Known risk to watch during execution:** COM threading. Device enumeration is deliberately done on the main goroutine (where `main` called `CoInitializeEx`), not on the Gio goroutine, to match today's working behavior. The `pinVolume` goroutine keeps calling `setAudioVolume` exactly as the old code did. If enumeration returns a COM "not initialized" error at runtime on Windows, initialize COM on the enumerating goroutine — but do not change the pinning goroutine, which already works.
