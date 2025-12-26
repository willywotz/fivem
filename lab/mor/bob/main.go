package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/kbinani/screenshot"
)

func main() {
	// http.HandleFunc("/", IndexHandler)

	// log.Println("server listening on :8080")
	// log.Println(http.ListenAndServe(":8080", nil))

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	n := 0
	t := time.NewTicker(5 * time.Second)
	c := make(chan struct{}, 1)

	go func() {
		for {
			capture()
			c <- struct{}{}
		}
	}()

	for {
		select {
		case <-ch:
			return
		case <-c:
			n++
		case <-t.C:
			println("tick", n/5)
			n = 0
		}
	}
}

func capture() (image.Image, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, fmt.Errorf("no active displays found")
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	img, err := capture()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer

	if typeHeader := r.URL.Query().Get("type"); typeHeader == "png" {
		if err := png.Encode(&buf, img); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if typeHeader == "jpeg" {
		opts := jpeg.Options{
			Quality: 80,
		}

		if quality := r.URL.Query().Get("quality"); quality != "" {
			if q, err := strconv.Atoi(quality); err == nil {
				opts.Quality = q
			}
		}

		if err := jpeg.Encode(&buf, img, &opts); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]any{"image": buf.Bytes()})
}
