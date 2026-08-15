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
