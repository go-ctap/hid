//go:build linux

package hid

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHIDIOCFeature(t *testing.T) {
	require.Equal(t, uintptr(0xc0414806), hidIOCFeature(0x06, 65))
	require.Equal(t, uintptr(0xc0414807), hidIOCFeature(0x07, 65))
}
