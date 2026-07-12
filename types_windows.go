package hid

import (
	"sync"

	"golang.org/x/sys/windows"
)

type Device struct {
	hFile                   windows.Handle
	inputReportByteLength   uint16
	outputReportByteLength  uint16
	featureReportByteLength uint16
	readTimeout             uint32
	openAccess              uint32
	closeOnce               sync.Once
	closeErr                error
}
