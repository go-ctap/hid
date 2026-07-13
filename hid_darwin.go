package hid

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	cfRelease                 func(cfTypeRef)
	cfRetain                  func(cfTypeRef) cfTypeRef
	cfStringCreateWithCString func(cfTypeRef, uintptr, uint32) cfStringRef
	cfStringGetCString        func(cfStringRef, uintptr, cfIndex, uint32) bool
	cfStringGetLength         func(cfStringRef) cfIndex
	cfStringGetMaxSize        func(cfIndex, uint32) cfIndex
	cfNumberCreate            func(cfAllocatorRef, int32, uintptr) cfNumberRef
	cfNumberGetValue          func(cfTypeRef, int32, uintptr) bool
	cfDictionaryCreate        func(cfAllocatorRef, uintptr, uintptr, cfIndex, uintptr, uintptr) cfDictionaryRef
	cfGetTypeID               func(cfTypeRef) uintptr
	cfStringGetTypeID         func() uintptr
	cfSetGetCount             func(cfSetRef) cfIndex
	cfSetGetValues            func(cfSetRef, uintptr)
	cfRunLoopGetCurrent       func() uintptr
	cfRunLoopRunInMode        func(cfStringRef, float64, bool) int32
	cfRunLoopStop             func(uintptr)

	ioHIDManagerCreate                         func(cfAllocatorRef, ioOptionBits) ioHIDManagerRef
	ioHIDManagerSetDeviceMatching              func(ioHIDManagerRef, cfDictionaryRef)
	ioHIDManagerCopyDevices                    func(ioHIDManagerRef) cfSetRef
	ioHIDManagerRegisterDeviceMatchingCallback func(ioHIDManagerRef, uintptr, uintptr)
	ioHIDManagerRegisterDeviceRemovalCallback  func(ioHIDManagerRef, uintptr, uintptr)
	ioHIDManagerScheduleWithRunLoop            func(ioHIDManagerRef, uintptr, cfStringRef)
	ioHIDManagerUnscheduleFromRunLoop          func(ioHIDManagerRef, uintptr, cfStringRef)
	ioHIDDeviceOpen                            func(ioHIDDeviceRef, ioOptionBits) ioReturn
	ioHIDDeviceClose                           func(ioHIDDeviceRef, ioOptionBits) ioReturn
	ioHIDDeviceGetProperty                     func(ioHIDDeviceRef, cfStringRef) cfTypeRef
	ioHIDDeviceGetService                      func(ioHIDDeviceRef) ioServiceRef
	ioHIDDeviceRegisterInputReportCallback     func(ioHIDDeviceRef, uintptr, cfIndex, uintptr, uintptr)
	ioHIDDeviceScheduleWithRunLoop             func(ioHIDDeviceRef, uintptr, cfStringRef)
	ioHIDDeviceUnscheduleFromRunLoop           func(ioHIDDeviceRef, uintptr, cfStringRef)
	ioHIDDeviceSetReport                       func(ioHIDDeviceRef, ioHIDReportType, cfIndex, []byte, cfIndex) ioReturn
	ioHIDDeviceGetReport                       func(ioHIDDeviceRef, ioHIDReportType, cfIndex, []byte, *cfIndex) ioReturn
	ioRegistryEntryGetRegistryEntryID          func(ioServiceRef, uintptr) ioReturn

	cfRunLoopDefaultMode cfStringRef
	cfDictionaryKeyCB    uintptr
	cfDictionaryValueCB  uintptr

	inputReportCallbackPtr = purego.NewCallback(inputReportCallback)
	activeDevicesMu        sync.RWMutex
	activeDevices          = make(map[uintptr]*Device)
	deviceSeq              atomic.Uint64
)

func init() {
	coreFoundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	ioKit, err := purego.Dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}

	purego.RegisterLibFunc(&cfRelease, coreFoundation, "CFRelease")
	purego.RegisterLibFunc(&cfRetain, coreFoundation, "CFRetain")
	purego.RegisterLibFunc(&cfStringCreateWithCString, coreFoundation, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&cfStringGetCString, coreFoundation, "CFStringGetCString")
	purego.RegisterLibFunc(&cfStringGetLength, coreFoundation, "CFStringGetLength")
	purego.RegisterLibFunc(&cfStringGetMaxSize, coreFoundation, "CFStringGetMaximumSizeForEncoding")
	purego.RegisterLibFunc(&cfNumberCreate, coreFoundation, "CFNumberCreate")
	purego.RegisterLibFunc(&cfNumberGetValue, coreFoundation, "CFNumberGetValue")
	purego.RegisterLibFunc(&cfDictionaryCreate, coreFoundation, "CFDictionaryCreate")
	purego.RegisterLibFunc(&cfGetTypeID, coreFoundation, "CFGetTypeID")
	purego.RegisterLibFunc(&cfStringGetTypeID, coreFoundation, "CFStringGetTypeID")
	purego.RegisterLibFunc(&cfSetGetCount, coreFoundation, "CFSetGetCount")
	purego.RegisterLibFunc(&cfSetGetValues, coreFoundation, "CFSetGetValues")
	purego.RegisterLibFunc(&cfRunLoopGetCurrent, coreFoundation, "CFRunLoopGetCurrent")
	purego.RegisterLibFunc(&cfRunLoopRunInMode, coreFoundation, "CFRunLoopRunInMode")
	purego.RegisterLibFunc(&cfRunLoopStop, coreFoundation, "CFRunLoopStop")

	purego.RegisterLibFunc(&ioHIDManagerCreate, ioKit, "IOHIDManagerCreate")
	purego.RegisterLibFunc(&ioHIDManagerSetDeviceMatching, ioKit, "IOHIDManagerSetDeviceMatching")
	purego.RegisterLibFunc(&ioHIDManagerCopyDevices, ioKit, "IOHIDManagerCopyDevices")
	purego.RegisterLibFunc(&ioHIDManagerRegisterDeviceMatchingCallback, ioKit, "IOHIDManagerRegisterDeviceMatchingCallback")
	purego.RegisterLibFunc(&ioHIDManagerRegisterDeviceRemovalCallback, ioKit, "IOHIDManagerRegisterDeviceRemovalCallback")
	purego.RegisterLibFunc(&ioHIDManagerScheduleWithRunLoop, ioKit, "IOHIDManagerScheduleWithRunLoop")
	purego.RegisterLibFunc(&ioHIDManagerUnscheduleFromRunLoop, ioKit, "IOHIDManagerUnscheduleFromRunLoop")
	purego.RegisterLibFunc(&ioHIDDeviceOpen, ioKit, "IOHIDDeviceOpen")
	purego.RegisterLibFunc(&ioHIDDeviceClose, ioKit, "IOHIDDeviceClose")
	purego.RegisterLibFunc(&ioHIDDeviceGetProperty, ioKit, "IOHIDDeviceGetProperty")
	purego.RegisterLibFunc(&ioHIDDeviceGetService, ioKit, "IOHIDDeviceGetService")
	purego.RegisterLibFunc(&ioHIDDeviceRegisterInputReportCallback, ioKit, "IOHIDDeviceRegisterInputReportCallback")
	purego.RegisterLibFunc(&ioHIDDeviceScheduleWithRunLoop, ioKit, "IOHIDDeviceScheduleWithRunLoop")
	purego.RegisterLibFunc(&ioHIDDeviceUnscheduleFromRunLoop, ioKit, "IOHIDDeviceUnscheduleFromRunLoop")
	purego.RegisterLibFunc(&ioHIDDeviceSetReport, ioKit, "IOHIDDeviceSetReport")
	purego.RegisterLibFunc(&ioHIDDeviceGetReport, ioKit, "IOHIDDeviceGetReport")
	purego.RegisterLibFunc(&ioRegistryEntryGetRegistryEntryID, ioKit, "IORegistryEntryGetRegistryEntryID")

	cfRunLoopDefaultMode = cfString(kCFRunLoopDefaultMode)
	cfDictionaryKeyCB, err = purego.Dlsym(coreFoundation, "kCFTypeDictionaryKeyCallBacks")
	if err != nil {
		panic(err)
	}
	cfDictionaryValueCB, err = purego.Dlsym(coreFoundation, "kCFTypeDictionaryValueCallBacks")
	if err != nil {
		panic(err)
	}
}

func Enumerate(options ...EnumerateOption) iter.Seq2[*DeviceInfo, error] {
	opts := newEnumerateOptions(options)

	return func(yield func(*DeviceInfo, error) bool) {
		if err := withDevices(&opts, func(device ioHIDDeviceRef) (bool, error) {
			info, err := getDeviceInfo(device)
			if err != nil {
				return yield(nil, err), nil
			}
			if !opts.match(info) {
				return true, nil
			}
			return yield(info, nil), nil
		}); err != nil {
			yield(nil, err)
		}
	}
}

func OpenPath(path string) (*Device, error) {
	var opened *Device

	if err := withDevices(nil, func(device ioHIDDeviceRef) (bool, error) {
		info, err := getDeviceInfo(device)
		if err != nil {
			return true, nil
		}
		if info.Path != path {
			return true, nil
		}

		if ret := ioHIDDeviceOpen(device, 0); ret != kIOReturnSuccess {
			return false, ioReturnError("IOHIDDeviceOpen", ret)
		}

		d := &Device{
			device:                 ioHIDDeviceRef(cfRetain(cfTypeRef(device))),
			inputReportByteLength:  intProperty(device, "MaxInputReportSize"),
			outputReportByteLength: intProperty(device, "MaxOutputReportSize"),
			reports:                make(chan []byte, 16),
			ready:                  make(chan struct{}),
			stopped:                make(chan struct{}),
		}
		if d.inputReportByteLength <= 0 {
			d.inputReportByteLength = 64
		}
		if d.outputReportByteLength <= 0 {
			d.outputReportByteLength = 64
		}
		d.inputReportBuffer = make([]byte, d.inputReportByteLength)
		d.inputReportBufferPin.Pin(unsafe.SliceData(d.inputReportBuffer))

		cbID := uintptr(deviceSeq.Add(1))
		d.cbID = cbID
		registerDevice(cbID, d)

		go d.run()
		<-d.ready

		opened = d
		return false, nil
	}); err != nil {
		return nil, err
	}

	if opened == nil {
		return nil, errors.New("device not found")
	}

	return opened, nil
}

func (d *Device) Read(ctx context.Context, p []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	select {
	case <-ctx.Done():
		return 0, ctx.Err()

	case report, ok := <-d.reports:
		if !ok {
			return 0, errors.New("device closed")
		}

		return copy(p, report), nil
	}
}

func (d *Device) Write(ctx context.Context, p []byte) (int, error) {
	reportID, report := prepareOutputReport(p, d.outputReportByteLength)
	inputLength := len(p)

	result := runIO(ctx, &d.writeMu, func() ioResult {
		if ret := ioHIDDeviceSetReport(
			d.device,
			kIOHIDReportTypeOutput,
			reportID,
			report,
			cfIndex(len(report)),
		); ret != kIOReturnSuccess {
			return ioResult{err: ioReturnError("IOHIDDeviceSetReport", ret)}
		}

		return ioResult{n: inputLength}
	})

	return result.n, result.err
}

func (d *Device) SendFeatureReport(report []byte) error {
	reportID, data := prepareIOHIDReport(report)

	ret := ioHIDDeviceSetReport(
		d.device,
		kIOHIDReportTypeFeature,
		reportID,
		data,
		cfIndex(len(data)),
	)
	if ret != kIOReturnSuccess {
		return ioReturnError("IOHIDDeviceSetReport", ret)
	}

	return nil
}

func (d *Device) GetFeatureReport(report []byte) (int, error) {
	reportID, data := prepareIOHIDReport(report)
	length := cfIndex(len(data))

	ret := ioHIDDeviceGetReport(
		d.device,
		kIOHIDReportTypeFeature,
		reportID,
		data,
		&length,
	)
	if ret != kIOReturnSuccess {
		return 0, ioReturnError("IOHIDDeviceGetReport", ret)
	}

	n := int(length)
	if reportID == 0 {
		n++
	}
	return n, nil
}

func prepareIOHIDReport(report []byte) (cfIndex, []byte) {
	reportID := cfIndex(0)
	if len(report) > 0 {
		reportID = cfIndex(report[0])
		if reportID == 0 {
			// IOHID receives an unnumbered report without the synthetic leading zero.
			report = report[1:]
		}
	}
	return reportID, report
}

func prepareOutputReport(p []byte, reportByteLength int) (cfIndex, []byte) {
	reportID, input := prepareIOHIDReport(p)
	report := make([]byte, reportByteLength)
	if len(input) > len(report) {
		input = input[:len(report)]
	}
	copy(report, input)

	return reportID, report
}

func (d *Device) Close() error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return nil
	}
	d.closed = true
	if d.runLoop != 0 {
		cfRunLoopStop(d.runLoop)
	}
	d.closeMu.Unlock()

	<-d.stopped
	unregisterDevice(d.cbID)
	_ = ioHIDDeviceClose(d.device, 0)
	cfRelease(cfTypeRef(d.device))
	runtime.KeepAlive(d.inputReportBuffer)
	d.inputReportBufferPin.Unpin()
	return nil
}

func (d *Device) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	d.runLoop = cfRunLoopGetCurrent()
	buf := d.inputReportBuffer
	ioHIDDeviceRegisterInputReportCallback(
		d.device,
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))),
		cfIndex(len(buf)),
		inputReportCallbackPtr,
		d.cbID,
	)
	ioHIDDeviceScheduleWithRunLoop(d.device, d.runLoop, cfRunLoopDefaultMode)
	close(d.ready)

	for {
		d.closeMu.Lock()
		closed := d.closed
		d.closeMu.Unlock()
		if closed {
			break
		}

		// A finite timeout also covers Close racing with the first run-loop call.
		if cfRunLoopRunInMode(cfRunLoopDefaultMode, 1, false) == kCFRunLoopRunFinished {
			break
		}
	}

	ioHIDDeviceUnscheduleFromRunLoop(d.device, d.runLoop, cfRunLoopDefaultMode)
	d.closeMu.Lock()
	d.runLoop = 0
	d.closeMu.Unlock()
	close(d.reports)
	close(d.stopped)
}

func ioReturnError(operation string, result ioReturn) error {
	if result == kIOReturnNotPermitted || result == kIOReturnNotPrivileged {
		return fmt.Errorf("%w: %s failed: 0x%08x", os.ErrPermission, operation, uint32(result))
	}
	return fmt.Errorf("%s failed: 0x%08x", operation, uint32(result))
}

func inputReportCallback(context uintptr, result ioReturn, sender uintptr, reportType ioHIDReportType, reportID uint32, report unsafe.Pointer, reportLength cfIndex) {
	if result != kIOReturnSuccess || reportLength <= 0 {
		return
	}
	d, ok := deviceByCallbackID(context)
	if !ok {
		return
	}

	data := unsafe.Slice((*byte)(report), int(reportLength))
	copied := bytes.Clone(data)

	select {
	case d.reports <- copied:
	default:
	}
}

func registerDevice(id uintptr, device *Device) {
	activeDevicesMu.Lock()
	activeDevices[id] = device
	activeDevicesMu.Unlock()
}

func unregisterDevice(id uintptr) {
	activeDevicesMu.Lock()
	delete(activeDevices, id)
	activeDevicesMu.Unlock()
}

func deviceByCallbackID(id uintptr) (*Device, bool) {
	activeDevicesMu.RLock()
	device, ok := activeDevices[id]
	activeDevicesMu.RUnlock()
	return device, ok
}

func withDevices(opts *enumerateOptions, yield func(ioHIDDeviceRef) (bool, error)) error {
	manager := ioHIDManagerCreate(kCFAllocatorDefault, kIOHIDManagerOptionNone)
	if manager == 0 {
		return errors.New("IOHIDManagerCreate failed")
	}
	defer cfRelease(cfTypeRef(manager))

	matching := deviceMatching(opts)
	if matching != 0 {
		defer cfRelease(cfTypeRef(matching))
	}
	ioHIDManagerSetDeviceMatching(manager, matching)

	deviceSet := ioHIDManagerCopyDevices(manager)
	if deviceSet == 0 {
		return nil
	}
	defer cfRelease(cfTypeRef(deviceSet))

	count := int(cfSetGetCount(deviceSet))
	if count == 0 {
		return nil
	}

	values := make([]uintptr, count)
	cfSetGetValues(deviceSet, uintptr(unsafe.Pointer(unsafe.SliceData(values))))

	for _, value := range values {
		if value == 0 {
			continue
		}
		next, err := yield(ioHIDDeviceRef(value))
		if err != nil {
			return err
		}
		if !next {
			return nil
		}
	}

	return nil
}

func deviceMatching(opts *enumerateOptions) cfDictionaryRef {
	if opts == nil {
		return 0
	}

	keys := make([]cfTypeRef, 0, 9)
	values := make([]cfTypeRef, 0, 9)
	defer func() {
		for _, key := range keys {
			cfRelease(key)
		}
		for _, value := range values {
			cfRelease(value)
		}
	}()

	addNumber := func(key string, value int32) bool {
		cfKey := cfString(key)
		if cfKey == 0 {
			return false
		}

		cfValue := cfNumber(value)
		if cfValue == 0 {
			cfRelease(cfTypeRef(cfKey))
			return false
		}

		keys = append(keys, cfTypeRef(cfKey))
		values = append(values, cfTypeRef(cfValue))
		return true
	}
	addString := func(key, value string) bool {
		cfKey := cfString(key)
		if cfKey == 0 {
			return false
		}

		cfValue := cfString(value)
		if cfValue == 0 {
			cfRelease(cfTypeRef(cfKey))
			return false
		}

		keys = append(keys, cfTypeRef(cfKey))
		values = append(values, cfTypeRef(cfValue))
		return true
	}

	if opts.vendorID != nil {
		if !addNumber("VendorID", int32(*opts.vendorID)) {
			return 0
		}
	}
	if opts.productID != nil {
		if !addNumber("ProductID", int32(*opts.productID)) {
			return 0
		}
	}
	if opts.serialNbr != nil {
		if !addString("SerialNumber", *opts.serialNbr) {
			return 0
		}
	}
	if opts.releaseNbr != nil {
		if !addNumber("VersionNumber", int32(*opts.releaseNbr)) {
			return 0
		}
	}
	if opts.mfrStr != nil {
		if !addString("Manufacturer", *opts.mfrStr) {
			return 0
		}
	}
	if opts.productStr != nil {
		if !addString("Product", *opts.productStr) {
			return 0
		}
	}
	if opts.usagePage != nil {
		if !addNumber("PrimaryUsagePage", int32(*opts.usagePage)) {
			return 0
		}
	}
	if opts.usage != nil {
		if !addNumber("PrimaryUsage", int32(*opts.usage)) {
			return 0
		}
	}
	if len(keys) == 0 {
		return 0
	}

	return cfDictionaryCreate(
		kCFAllocatorDefault,
		uintptr(unsafe.Pointer(unsafe.SliceData(keys))),
		uintptr(unsafe.Pointer(unsafe.SliceData(values))),
		cfIndex(len(keys)),
		cfDictionaryKeyCB,
		cfDictionaryValueCB,
	)
}

func getDeviceInfo(device ioHIDDeviceRef) (*DeviceInfo, error) {
	entryID, err := registryEntryID(device)
	if err != nil {
		return nil, err
	}

	return &DeviceInfo{
		Path:       strconv.FormatUint(entryID, 16),
		VendorID:   uint16(intProperty(device, "VendorID")),
		ProductID:  uint16(intProperty(device, "ProductID")),
		ReleaseNbr: uint16(intProperty(device, "VersionNumber")),
		SerialNbr:  stringProperty(device, "SerialNumber"),
		MfrStr:     stringProperty(device, "Manufacturer"),
		ProductStr: stringProperty(device, "Product"),
		UsagePage:  uint16(intProperty(device, "PrimaryUsagePage")),
		Usage:      uint16(intProperty(device, "PrimaryUsage")),
		InstanceID: strconv.FormatUint(entryID, 16),
	}, nil
}

func registryEntryID(device ioHIDDeviceRef) (uint64, error) {
	service := ioHIDDeviceGetService(device)
	if service == 0 {
		return 0, errors.New("IOHIDDeviceGetService failed")
	}

	var id uint64
	if ret := ioRegistryEntryGetRegistryEntryID(service, uintptr(unsafe.Pointer(&id))); ret != kIOReturnSuccess {
		return 0, fmt.Errorf("IORegistryEntryGetRegistryEntryID failed: 0x%x", int32(ret))
	}
	return id, nil
}

func intProperty(device ioHIDDeviceRef, key string) int {
	cfKey := cfString(key)
	defer cfRelease(cfTypeRef(cfKey))

	value := ioHIDDeviceGetProperty(device, cfKey)
	if value == 0 {
		return 0
	}

	var n int32
	if !cfNumberGetValue(value, kCFNumberSInt32Type, uintptr(unsafe.Pointer(&n))) {
		return 0
	}
	return int(n)
}

func stringProperty(device ioHIDDeviceRef, key string) string {
	cfKey := cfString(key)
	defer cfRelease(cfTypeRef(cfKey))

	value := ioHIDDeviceGetProperty(device, cfKey)
	if value == 0 || cfGetTypeID(value) != cfStringGetTypeID() {
		return ""
	}
	return cfStringToString(cfStringRef(value))
}

func cfString(s string) cfStringRef {
	buf := append([]byte(s), 0)
	return cfStringCreateWithCString(0, uintptr(unsafe.Pointer(unsafe.SliceData(buf))), kCFStringEncodingUTF8)
}

func cfNumber(n int32) cfNumberRef {
	return cfNumberCreate(kCFAllocatorDefault, kCFNumberSInt32Type, uintptr(unsafe.Pointer(&n)))
}

func cfStringToString(s cfStringRef) string {
	if s == 0 {
		return ""
	}

	bufSize := cfStringGetMaxSize(cfStringGetLength(s), kCFStringEncodingUTF8) + 1
	if bufSize <= 1 {
		return ""
	}

	buf := make([]byte, int(bufSize))
	if !cfStringGetCString(s, uintptr(unsafe.Pointer(unsafe.SliceData(buf))), cfIndex(len(buf)), kCFStringEncodingUTF8) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}
