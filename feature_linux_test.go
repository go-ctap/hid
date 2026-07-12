//go:build linux

package hid

import (
	"testing"
)

func TestHIDIOCFeature(t *testing.T) {
	if got, want := hidIOCFeature(0x06, 65), uintptr(0xc0414806); got != want {
		t.Errorf("hidIOCFeature(0x06, 65) = %#x, want %#x", got, want)
	}
	if got, want := hidIOCFeature(0x07, 65), uintptr(0xc0414807); got != want {
		t.Errorf("hidIOCFeature(0x07, 65) = %#x, want %#x", got, want)
	}
}
