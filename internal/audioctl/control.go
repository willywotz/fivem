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
