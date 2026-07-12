//go:build linux

package hid

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFeatureReportBuffer(t *testing.T) {
	require.NoError(t, validateFeatureReportBuffer([]byte{0}))

	require.Error(t, validateFeatureReportBuffer(nil))
	require.Error(t, validateFeatureReportBuffer(make([]byte, 1<<14)))
}

func TestHIDIOCFeature(t *testing.T) {
	require.NotZero(t, hidIOCFeature(0x06, 65))
	require.NotEqual(t, hidIOCFeature(0x06, 65), hidIOCFeature(0x07, 65))
}
