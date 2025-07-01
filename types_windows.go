package hid

import "golang.org/x/sys/windows"

type Device struct {
	hFile                   windows.Handle
	overlapped              *windows.Overlapped
	inputReportByteLength   uint16
	outputReportByteLength  uint16
	featureReportByteLength uint16
}
