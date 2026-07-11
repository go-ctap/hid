//go:build darwin

package hid

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestInputReportCallbackPreservesLeadingZero(t *testing.T) {
	device := &Device{
		reports: make(chan []byte, 1),
	}
	callbackID := uintptr(deviceSeq.Add(1))
	registerDevice(callbackID, device)
	t.Cleanup(func() {
		unregisterDevice(callbackID)
	})

	report := make([]byte, 64)
	for i := range report {
		report[i] = byte(i)
	}

	// For an unnumbered report, the leading zero belongs to the payload.
	inputReportCallback(
		callbackID,
		kIOReturnSuccess,
		0,
		kIOHIDReportTypeInput,
		0,
		unsafe.Pointer(unsafe.SliceData(report)),
		cfIndex(len(report)),
	)

	select {
	case received := <-device.reports:
		require.Equal(t, report, received)
	default:
		t.Fatal("input report callback did not enqueue the report")
	}
}

func TestDeviceWriteFormatsReportID(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		wantID     cfIndex
		wantReport []byte
	}{
		{
			name:       "full unnumbered report",
			input:      []byte{0, 1, 2, 3, 4},
			wantID:     0,
			wantReport: []byte{1, 2, 3, 4},
		},
		{
			name:       "numbered report",
			input:      []byte{5, 1, 2, 3},
			wantID:     5,
			wantReport: []byte{5, 1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotReport := prepareOutputReport(tt.input, len(tt.wantReport))

			require.Equal(t, tt.wantID, gotID)
			require.Equal(t, tt.wantReport, gotReport)
		})
	}
}
