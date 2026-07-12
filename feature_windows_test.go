//go:build windows

package hid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureReportBufferPadsToDeviceLength(t *testing.T) {
	dev := &Device{featureReportByteLength: 5}

	buf, err := dev.featureReportBuffer([]byte{0, 1, 2})
	require.NoError(t, err)

	assert.Equal(t, []byte{0, 1, 2, 0, 0}, buf)
}

func TestFeatureReportBufferRejectsInvalidLengths(t *testing.T) {
	dev := &Device{featureReportByteLength: 2}

	_, err := dev.featureReportBuffer(nil)
	require.Error(t, err)

	_, err = dev.featureReportBuffer([]byte{0, 1, 2})
	require.Error(t, err)
}

func TestFeatureReportBufferUsesCallerLengthWhenDeviceCapsReportZero(t *testing.T) {
	dev := &Device{featureReportByteLength: 0}

	buf, err := dev.featureReportBuffer([]byte{0, 1, 2})
	require.NoError(t, err)

	assert.Equal(t, []byte{0, 1, 2}, buf)
}
