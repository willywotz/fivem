package main

import (
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	ole "github.com/go-ole/go-ole"
	"github.com/willywotz/fivem/internal/audioctl"
)

type audioInit struct {
	devices []AudioDevice
	err     error
}

// applyStatus holds the last volume-apply error so the UI can show it.
type applyStatus struct {
	mu  sync.Mutex
	err string
}

func (s *applyStatus) set(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.err = err.Error()
	} else {
		s.err = ""
	}
}

func (s *applyStatus) get() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func ui() {
	control := audioctl.New()
	status := &applyStatus{}

	initCh := make(chan audioInit, 1)
	go audioWorker(control, status, initCh)
	init := <-initCh
	if init.err != nil {
		slog.Error("error getting audio input devices", "err", init.err)
	}

	go func() {
		w := new(app.Window)
		w.Option(app.Title("fivem tools"), app.Size(unit.Dp(480), unit.Dp(320)))
		if err := runUI(w, init.devices, control, status, init.err); err != nil {
			slog.Error("ui loop failed", "err", err)
		}
		os.Exit(0)
	}()

	app.Main()
}

// audioWorker owns a COM apartment on a locked OS thread and does all WASAPI
// work: it enumerates the input devices once and reports them, then pins the
// selected volume every 100 ms. WASAPI needs COM initialized on the calling
// thread, so all of it must live on this one thread.
func audioWorker(c *audioctl.Control, status *applyStatus, out chan<- audioInit) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// go-ole reports any non-zero HRESULT as an error, but two are benign:
	// S_FALSE means COM is already initialized on this thread (LockOSThread can
	// pin us to an already-initialized thread), and RPC_E_CHANGED_MODE means the
	// thread is already in another apartment. COM is usable in both cases.
	const sFalse, rpcChangedMode = uintptr(0x00000001), uintptr(0x80010106)
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		code := uintptr(0)
		if oleErr, ok := err.(*ole.OleError); ok {
			code = oleErr.Code()
		}
		if code != sFalse && code != rpcChangedMode {
			out <- audioInit{err: fmt.Errorf("failed to initialize COM: %w", err)}
			return
		}
	}
	defer ole.CoUninitialize()

	devices, err := getAudioInputDevices()
	for _, d := range devices {
		if d.IsDefaultAudioEndpoint {
			c.SetEndpoint(d.ID)
		}
	}
	out <- audioInit{devices: devices, err: err}

	for range time.Tick(100 * time.Millisecond) {
		id, v := c.Snapshot()
		if id == "" || v < 0 || v > 1 {
			continue
		}
		err := setAudioVolume(id, v)
		status.set(err)
		if err != nil {
			slog.Error("error setting volume", "err", err)
		}
	}
}

func runUI(w *app.Window, devices []AudioDevice, c *audioctl.Control, status *applyStatus, loadErr error) error {
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
					layout.Rigid(material.Body1(th, "Volume: "+strconv.Itoa(int(vol.Value*100))+"%").Layout),
					layout.Rigid(material.Slider(th, &vol).Layout),
					// Apply status: shows the last setAudioVolume error, if any.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						msg := status.get()
						if msg == "" {
							return layout.Dimensions{}
						}
						l := material.Body2(th, "Apply error: "+msg)
						l.Color = color.NRGBA{R: 0xB0, A: 0xFF}
						return l.Layout(gtx)
					}),
				)
			})

			e.Frame(gtx.Ops)
		}
	}
}
