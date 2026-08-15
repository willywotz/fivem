package main

import (
	"image/color"
	"log/slog"
	"os"
	"strconv"
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
					layout.Rigid(material.Body1(th, "Volume: "+strconv.Itoa(int(vol.Value*100))+"%").Layout),
					layout.Rigid(material.Slider(th, &vol).Layout),
				)
			})

			e.Frame(gtx.Ops)
		}
	}
}
