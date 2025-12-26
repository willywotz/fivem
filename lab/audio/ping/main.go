package main

import (
	"encoding/json"
	"os"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

func main() {
	pinger, err := probing.NewPinger("212.80.214.124")
	if err != nil {
		panic(err)
	}
	pinger.Interval = 100 * time.Millisecond
	pinger.SetPrivileged(true)
	pinger.RecordRtts = false
	pinger.RecordTTLs = false
	go func() {
		for range time.Tick(1 * time.Second) {
			_ = json.NewEncoder(os.Stdout).Encode(pinger.Statistics())
		}
	}()
	if err := pinger.Run(); err != nil {
		panic(err)
	}
}
