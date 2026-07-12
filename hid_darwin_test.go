//go:build darwin

package hid

import (
	"slices"
	"testing"
	"unsafe"
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
		if !slices.Equal(received, report) {
			t.Fatalf("received report = %v, want %v", received, report)
		}
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

			if gotID != tt.wantID {
				t.Errorf("report ID = %d, want %d", gotID, tt.wantID)
			}
			if !slices.Equal(gotReport, tt.wantReport) {
				t.Errorf("report = %v, want %v", gotReport, tt.wantReport)
			}
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

			if err := device.SendFeatureReport(report); err != nil {
				t.Fatal(err)
			}
			if got.device != ioHIDDeviceRef(0x1234) {
				t.Errorf("device = %#x, want %#x", got.device, ioHIDDeviceRef(0x1234))
			}
			if got.reportType != kIOHIDReportTypeFeature {
				t.Errorf("report type = %d, want %d", got.reportType, kIOHIDReportTypeFeature)
			}
			if got.reportID != tt.wantID {
				t.Errorf("report ID = %d, want %d", got.reportID, tt.wantID)
			}
			if !slices.Equal(got.data, tt.wantData) {
				t.Errorf("data = %v, want %v", got.data, tt.wantData)
			}
			if got.length != tt.wantLength {
				t.Errorf("length = %d, want %d", got.length, tt.wantLength)
			}
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
			if err != nil {
				t.Fatal(err)
			}
			if n != tt.wantN {
				t.Errorf("GetFeatureReport() = %d bytes, want %d", n, tt.wantN)
			}
			if !slices.Equal(report, tt.wantReport) {
				t.Errorf("report = %v, want %v", report, tt.wantReport)
			}
			if got.device != ioHIDDeviceRef(0x1234) {
				t.Errorf("device = %#x, want %#x", got.device, ioHIDDeviceRef(0x1234))
			}
			if got.reportType != kIOHIDReportTypeFeature {
				t.Errorf("report type = %d, want %d", got.reportType, kIOHIDReportTypeFeature)
			}
			if got.reportID != tt.wantID {
				t.Errorf("report ID = %d, want %d", got.reportID, tt.wantID)
			}
			if got.dataLength != int(tt.wantBufferLength) {
				t.Errorf("data length = %d, want %d", got.dataLength, tt.wantBufferLength)
			}
			if got.bufferLength != tt.wantBufferLength {
				t.Errorf("buffer length = %d, want %d", got.bufferLength, tt.wantBufferLength)
			}
		})
	}
}
