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
	readMu                  sync.Mutex
	writeMu                 sync.Mutex
	closeOnce               sync.Once
	closeErr                error
}
