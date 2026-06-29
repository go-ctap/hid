package hid

import "sync"

const (
	kCFStringEncodingUTF8                = 0x08000100
	kCFNumberSInt32Type                  = 3
	kCFAllocatorDefault   cfAllocatorRef = 0x0

	kIOHIDManagerOptionNone ioOptionBits    = 0x0
	kIOHIDReportTypeInput   ioHIDReportType = 0
	kIOHIDReportTypeOutput  ioHIDReportType = 1

	kIOReturnSuccess ioReturn = 0

	kCFRunLoopDefaultMode = "kCFRunLoopDefaultMode"
)

type (
	cfAllocatorRef  uintptr
	cfDictionaryRef uintptr
	cfNumberRef     uintptr
	cfTypeRef       uintptr
	cfStringRef     uintptr
	cfSetRef        uintptr
	ioHIDManagerRef uintptr
	ioHIDDeviceRef  uintptr
	ioServiceRef    uintptr
	cfIndex         int64
	ioReturn        int32
	ioOptionBits    uint32
	ioHIDReportType int32
)

type Device struct {
	device                 ioHIDDeviceRef
	inputReportByteLength  int
	outputReportByteLength int

	reports chan []byte
	ready   chan struct{}
	stopped chan struct{}

	runLoop uintptr
	cbID    uintptr
	closeMu sync.Mutex
	closed  bool
}
