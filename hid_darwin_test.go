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

func TestDeviceSendFeatureReportFormatsReportID(t *testing.T) {
	original := ioHIDDeviceSetReport
	t.Cleanup(func() {
		ioHIDDeviceSetReport = original
	})

	type nativeCall struct {
		device     ioHIDDeviceRef
		reportType ioHIDReportType
		reportID   cfIndex
		data       []byte
		length     cfIndex
	}
	var got nativeCall
	ioHIDDeviceSetReport = func(device ioHIDDeviceRef, reportType ioHIDReportType, reportID cfIndex, data []byte, length cfIndex) ioReturn {
		got = nativeCall{
			device:     device,
			reportType: reportType,
			reportID:   reportID,
			data:       append([]byte(nil), data...),
			length:     length,
		}
		return kIOReturnSuccess
	}

	device := &Device{device: 0x1234}
	tests := []struct {
		name       string
		report     []byte
		wantID     cfIndex
		wantData   []byte
		wantLength cfIndex
	}{
		{
			name:       "unnumbered report",
			report:     []byte{0, 0x10, 0x20, 0x30},
			wantID:     0,
			wantData:   []byte{0x10, 0x20, 0x30},
			wantLength: 3,
		},
		{
			name:       "numbered report",
			report:     []byte{5, 0x10, 0x20, 0x30},
			wantID:     5,
			wantData:   []byte{5, 0x10, 0x20, 0x30},
			wantLength: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got = nativeCall{}
			report := append([]byte(nil), tt.report...)

			require.NoError(t, device.SendFeatureReport(report))
			require.Equal(t, ioHIDDeviceRef(0x1234), got.device)
			require.Equal(t, kIOHIDReportTypeFeature, got.reportType)
			require.Equal(t, tt.wantID, got.reportID)
			require.Equal(t, tt.wantData, got.data)
			require.Equal(t, tt.wantLength, got.length)
		})
	}
}

func TestDeviceGetFeatureReportFormatsReportID(t *testing.T) {
	original := ioHIDDeviceGetReport
	t.Cleanup(func() {
		ioHIDDeviceGetReport = original
	})

	type nativeCall struct {
		device       ioHIDDeviceRef
		reportType   ioHIDReportType
		reportID     cfIndex
		dataLength   int
		bufferLength cfIndex
	}
	var (
		got      nativeCall
		report   []byte
		response []byte
	)
	ioHIDDeviceGetReport = func(device ioHIDDeviceRef, reportType ioHIDReportType, reportID cfIndex, data []byte, length *cfIndex) ioReturn {
		got = nativeCall{
			device:       device,
			reportType:   reportType,
			reportID:     reportID,
			dataLength:   len(data),
			bufferLength: *length,
		}
		*length = cfIndex(copy(data, response))
		return kIOReturnSuccess
	}

	device := &Device{device: 0x1234}
	tests := []struct {
		name             string
		initial          []byte
		response         []byte
		wantID           cfIndex
		wantBufferLength cfIndex
		wantReport       []byte
		wantN            int
	}{
		{
			name:             "unnumbered report",
			initial:          []byte{0, 0, 0, 0, 0},
			response:         []byte{0x10, 0x20, 0x30},
			wantID:           0,
			wantBufferLength: 4,
			wantReport:       []byte{0, 0x10, 0x20, 0x30, 0},
			wantN:            4,
		},
		{
			name:             "numbered report",
			initial:          []byte{5, 0, 0, 0, 0},
			response:         []byte{5, 0x10, 0x20},
			wantID:           5,
			wantBufferLength: 5,
			wantReport:       []byte{5, 0x10, 0x20, 0, 0},
			wantN:            3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got = nativeCall{}
			report = append([]byte(nil), tt.initial...)
			response = tt.response

			n, err := device.GetFeatureReport(report)
			require.NoError(t, err)
			require.Equal(t, tt.wantN, n)
			require.Equal(t, tt.wantReport, report)
			require.Equal(t, ioHIDDeviceRef(0x1234), got.device)
			require.Equal(t, kIOHIDReportTypeFeature, got.reportType)
			require.Equal(t, tt.wantID, got.reportID)
			require.Equal(t, int(tt.wantBufferLength), got.dataLength)
			require.Equal(t, tt.wantBufferLength, got.bufferLength)
		})
	}
}
