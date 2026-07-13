//go:generate powershell -Command "go tool cgo -godefs types_hid_windows.go | Set-Content -Path ztypes_hid_windows.go -Encoding UTF8"
package hid

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/unicode"
)

var (
	modHidsdi                            = windows.NewLazySystemDLL("hid.dll")
	procHidD_GetHidGuid                  = modHidsdi.NewProc("HidD_GetHidGuid")
	procHidD_GetAttributes               = modHidsdi.NewProc("HidD_GetAttributes")
	procHidD_GetManufacturerString       = modHidsdi.NewProc("HidD_GetManufacturerString")
	procHidD_GetProductString            = modHidsdi.NewProc("HidD_GetProductString")
	procHidD_GetSerialNumberString       = modHidsdi.NewProc("HidD_GetSerialNumberString")
	procHidD_GetFeature                  = modHidsdi.NewProc("HidD_GetFeature")
	procHidD_SetFeature                  = modHidsdi.NewProc("HidD_SetFeature")
	procHidD_GetPreparsedData            = modHidsdi.NewProc("HidD_GetPreparsedData")
	procHidD_FreePreparsedData           = modHidsdi.NewProc("HidD_FreePreparsedData")
	procHidP_GetCaps                     = modHidsdi.NewProc("HidP_GetCaps")
	modSetupapi                          = windows.NewLazySystemDLL("setupapi.dll")
	procSetupDiGetClassDevsW             = modSetupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiDestroyDeviceInfoList     = modSetupapi.NewProc("SetupDiDestroyDeviceInfoList")
	procSetupDiEnumDeviceInterfaces      = modSetupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInstanceIdW      = modSetupapi.NewProc("SetupDiGetDeviceInstanceIdW")
	procSetupDiGetDeviceInterfaceDetailW = modSetupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiGetDevicePropertyW        = modSetupapi.NewProc("SetupDiGetDevicePropertyW")
)

var (
	windowsReadFile            = windows.ReadFile
	windowsWriteFile           = windows.WriteFile
	windowsCancelIoEx          = windows.CancelIoEx
	windowsGetOverlappedResult = windows.GetOverlappedResult
)

func setupDiDestroyDeviceInfoList(deviceInfoSet windows.Handle) error {
	r1, _, err := procSetupDiDestroyDeviceInfoList.Call(uintptr(deviceInfoSet))
	if r1 == 0 {
		return err
	}

	return nil
}

func getHidGuid() (*windows.GUID, error) {
	var hidGuid windows.GUID
	_, _, err := procHidD_GetHidGuid.Call(
		uintptr(unsafe.Pointer(&hidGuid)),
	)
	if !errors.Is(err, windows.NOERROR) {
		return nil, err
	}

	return &hidGuid, nil
}

func getAttributes(hidDeviceObject windows.Handle) (*_HIDD_ATTRIBUTES, error) {
	var hidAttributes _HIDD_ATTRIBUTES
	hidAttributes.Size = uint32(unsafe.Sizeof(hidAttributes))
	r1, _, err := procHidD_GetAttributes.Call(
		uintptr(hidDeviceObject),
		uintptr(unsafe.Pointer(&hidAttributes)),
	)
	if r1 == 0 {
		return nil, err
	}

	return &hidAttributes, nil
}

func getManufacturerString(hidDeviceObject windows.Handle) ([]byte, error) {
	buf := make([]byte, 126*2)
	r1, _, err := procHidD_GetManufacturerString.Call(
		uintptr(hidDeviceObject),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r1 == 0 {
		return nil, err
	}

	return buf, nil
}

func getPreparsedData(hidDeviceObject windows.Handle) (_PHIDP_PREPARSED_DATA, error) {
	var preparsedData _PHIDP_PREPARSED_DATA
	r1, _, err := procHidD_GetPreparsedData.Call(
		uintptr(hidDeviceObject),
		uintptr(unsafe.Pointer(&preparsedData)),
	)
	if r1 == 0 {
		return 0, err
	}

	return preparsedData, nil
}

func freePreparsedData(preparsedData _PHIDP_PREPARSED_DATA) error {
	r1, _, err := procHidD_FreePreparsedData.Call(
		uintptr(preparsedData),
	)
	if r1 == 0 {
		return err
	}

	return nil
}

func getProductString(hidDeviceObject windows.Handle) ([]byte, error) {
	buf := make([]byte, 126*2)
	r1, _, err := procHidD_GetProductString.Call(
		uintptr(hidDeviceObject),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r1 == 0 {
		return nil, err
	}

	return bytes.Clone(buf), nil
}

func getSerialNumberString(hidDeviceObject windows.Handle) ([]byte, error) {
	buf := make([]byte, 126*2)
	r1, _, err := procHidD_GetSerialNumberString.Call(
		uintptr(hidDeviceObject),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r1 == 0 {
		return nil, err
	}

	return bytes.Clone(buf), nil
}

func getCaps(preparsedData _PHIDP_PREPARSED_DATA) (*_HIDP_CAPS, error) {
	var caps _HIDP_CAPS
	r1, _, err := procHidP_GetCaps.Call(
		uintptr(preparsedData),
		uintptr(unsafe.Pointer(&caps)),
	)
	if r1 != _HIDP_STATUS_SUCCESS {
		return nil, err
	}

	return &caps, nil
}

func setupDiGetClassDevs(
	guid *windows.GUID,
	enumerator string,
	hwndParent windows.Handle,
	flags windows.DIGCF,
) (windows.Handle, error) {
	var enumeratorW *uint16 = nil
	if enumerator != "" {
		enumeratorW = windows.StringToUTF16Ptr(enumerator)
	}

	r1, _, err := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(guid)),
		uintptr(unsafe.Pointer(enumeratorW)),
		uintptr(hwndParent),
		uintptr(flags),
	)
	if !errors.Is(err, windows.NOERROR) {
		return 0, err
	}

	return windows.Handle(r1), nil
}

func setupDiEnumDeviceInterfaces(
	deviceInfoSet windows.Handle,
	devInfoData *_SP_DEVINFO_DATA,
	interfaceClassGuid *windows.GUID,
	memberIndex uint32,
) (*_SP_DEVICE_INTERFACE_DATA, error) {
	var deviceInterfaceData _SP_DEVICE_INTERFACE_DATA
	deviceInterfaceData.CbSize = uint32(unsafe.Sizeof(deviceInterfaceData))

	r1, _, err := procSetupDiEnumDeviceInterfaces.Call(
		uintptr(deviceInfoSet),
		uintptr(unsafe.Pointer(devInfoData)),
		uintptr(unsafe.Pointer(interfaceClassGuid)),
		uintptr(memberIndex),
		uintptr(unsafe.Pointer(&deviceInterfaceData)),
	)
	if r1 == 0 {
		return nil, err
	}

	return &deviceInterfaceData, nil
}

func setupDiGetDeviceInstanceIdW(
	deviceSetInfo windows.Handle,
	devInfoData *_SP_DEVINFO_DATA,
) (string, error) {
	var requiredSize uint32
	r1, _, err := procSetupDiGetDeviceInstanceIdW.Call(
		uintptr(deviceSetInfo),
		uintptr(unsafe.Pointer(devInfoData)),
		0,
		0,
		uintptr(unsafe.Pointer(&requiredSize)),
	)
	if r1 == 0 && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return "", err
	}

	friendlyNameBuf := make([]uint16, requiredSize)
	r1, _, err = procSetupDiGetDeviceInstanceIdW.Call(
		uintptr(deviceSetInfo),
		uintptr(unsafe.Pointer(devInfoData)),
		uintptr(unsafe.Pointer(&friendlyNameBuf[0])),
		uintptr(requiredSize),
		uintptr(unsafe.Pointer(&requiredSize)),
	)
	if r1 == 0 {
		return "", err
	}

	return strings.Clone(windows.UTF16ToString(friendlyNameBuf)), nil
}

func setupDiGetDeviceInterfaceDetailW(
	deviceInfoSet windows.Handle,
	deviceInterfaceData *_SP_DEVICE_INTERFACE_DATA,
) (
	deviceInterfaceDetailData *_SP_DEVICE_INTERFACE_DETAIL_DATA_W,
	deviceInfoData *_SP_DEVINFO_DATA,
	err error,
) {
	var requiredSize uint32
	r1, _, err := procSetupDiGetDeviceInterfaceDetailW.Call(
		uintptr(deviceInfoSet),
		uintptr(unsafe.Pointer(deviceInterfaceData)),
		0,
		0,
		uintptr(unsafe.Pointer(&requiredSize)),
		uintptr(unsafe.Pointer(deviceInfoData)),
	)
	if r1 == 0 && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return nil, nil, err
	}

	detailDataBuf := make([]byte, requiredSize)
	deviceInterfaceDetailData = (*_SP_DEVICE_INTERFACE_DETAIL_DATA_W)(unsafe.Pointer(&detailDataBuf[0]))
	deviceInterfaceDetailData.CbSize = uint32(unsafe.Sizeof(*deviceInterfaceDetailData))
	deviceInfoData = new(_SP_DEVINFO_DATA)
	deviceInfoData.CbSize = uint32(unsafe.Sizeof(*deviceInfoData))

	r1, _, err = procSetupDiGetDeviceInterfaceDetailW.Call(
		uintptr(deviceInfoSet),
		uintptr(unsafe.Pointer(deviceInterfaceData)),
		uintptr(unsafe.Pointer(deviceInterfaceDetailData)),
		uintptr(requiredSize),
		uintptr(unsafe.Pointer(&requiredSize)),
		uintptr(unsafe.Pointer(deviceInfoData)),
	)
	if r1 == 0 {
		return nil, nil, err
	}

	return deviceInterfaceDetailData, deviceInfoData, nil
}

func setupDiGetDevicePropertyW(
	deviceInfoSet windows.Handle,
	deviceInfoData *_SP_DEVINFO_DATA,
	devPropKey *windows.DEVPROPKEY,
) (
	devPropType windows.DEVPROPTYPE,
	propertyBuffer []byte,
	err error,
) {
	var requiredSize uint32
	r1, _, err := procSetupDiGetDevicePropertyW.Call(
		uintptr(deviceInfoSet),
		uintptr(unsafe.Pointer(deviceInfoData)),
		uintptr(unsafe.Pointer(devPropKey)),
		uintptr(unsafe.Pointer(&devPropType)),
		0,
		0,
		uintptr(unsafe.Pointer(&requiredSize)),
		0,
	)
	if r1 == 0 && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return 0, nil, err
	}

	if requiredSize == 0 {
		return 0, nil, errors.New("invalid RequiredSize was returned")
	}
	propertyBuffer = make([]byte, requiredSize)

	r1, _, err = procSetupDiGetDevicePropertyW.Call(
		uintptr(deviceInfoSet),
		uintptr(unsafe.Pointer(deviceInfoData)),
		uintptr(unsafe.Pointer(devPropKey)),
		uintptr(unsafe.Pointer(&devPropType)),
		uintptr(unsafe.Pointer(&propertyBuffer[0])),
		uintptr(requiredSize),
		uintptr(unsafe.Pointer(&requiredSize)),
		0,
	)
	if r1 == 0 {
		return 0, nil, err
	}

	return devPropType, bytes.Clone(propertyBuffer), nil
}

func getDeviceInfo(devPath string) (*DeviceInfo, error) {
	devicePathPtr := windows.StringToUTF16Ptr(devPath)

	hFile, err := windows.CreateFile(
		devicePathPtr,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = windows.Close(hFile)
	}()

	deviceInfo := &DeviceInfo{
		Path: devPath,
	}

	attrs, err := getAttributes(hFile)
	if err != nil {
		return nil, err
	}
	deviceInfo.VendorID = attrs.VendorID
	deviceInfo.ProductID = attrs.ProductID
	deviceInfo.ReleaseNbr = attrs.VersionNumber

	decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()

	mfrStr, _ := getManufacturerString(hFile)
	if len(mfrStr) > 0 {
		deviceInfo.MfrStr, err = decoder.String(strings.TrimRight(string(mfrStr), string([]byte{0})) + "\u0000")
		if err != nil {
			return nil, err
		}
	}

	productStr, _ := getProductString(hFile)
	if len(productStr) > 0 {
		deviceInfo.ProductStr, err = decoder.String(strings.TrimRight(string(productStr), string([]byte{0})) + "\u0000")
		if err != nil {
			return nil, err
		}
	}

	serialNumberStr, _ := getSerialNumberString(hFile)
	if len(serialNumberStr) > 0 {
		deviceInfo.SerialNbr, err = decoder.String(strings.TrimRight(string(serialNumberStr), string([]byte{0})) + "\u0000")
		if err != nil {
			return nil, err
		}
	}

	if err := func() error {
		preparsedData, err := getPreparsedData(hFile)
		if err != nil {
			return err
		}
		defer func() {
			_ = freePreparsedData(preparsedData)
		}()

		caps, err := getCaps(preparsedData)
		if err != nil {
			return err
		}
		deviceInfo.UsagePage = caps.UsagePage
		deviceInfo.Usage = caps.Usage

		return nil
	}(); err != nil {
		return nil, err
	}

	return deviceInfo, nil
}

var (
	devpkeyDeviceDevNodeStatus = windows.DEVPROPKEY{
		FmtID: windows.DEVPROPGUID(windows.GUID{
			Data1: 0x4340a6c5,
			Data2: 0x93fa,
			Data3: 0x4706,
			Data4: [8]byte{0x97, 0x2c, 0x7b, 0x64, 0x80, 0x08, 0xa5, 0xa7},
		}),
		PID: 2,
	}
	devpkeyDeviceParent = windows.DEVPROPKEY{
		FmtID: windows.DEVPROPGUID(windows.GUID{
			Data1: 0x4340a6c5,
			Data2: 0x93fa,
			Data3: 0x4706,
			Data4: [8]byte{0x97, 0x2c, 0x7b, 0x64, 0x80, 0x08, 0xa5, 0xa7},
		}),
		PID: 8,
	}
)

func Enumerate(options ...EnumerateOption) iter.Seq2[*DeviceInfo, error] {
	opts := newEnumerateOptions(options)

	return func(yield func(deviceInfo *DeviceInfo, err error) bool) {
		guid, err := getHidGuid()
		if err != nil {
			yield(nil, err)
			return
		}

		deviceInfoSet, err := setupDiGetClassDevs(
			guid,
			"",
			0,
			windows.DIGCF_PRESENT|windows.DIGCF_DEVICEINTERFACE,
		)
		if err != nil {
			yield(nil, err)
			return
		}
		defer func() {
			_ = setupDiDestroyDeviceInfoList(deviceInfoSet)
		}()

		for interfaceMemberIndex := uint32(0); ; interfaceMemberIndex++ {
			deviceInterfaceData, err := setupDiEnumDeviceInterfaces(
				deviceInfoSet,
				nil,
				guid,
				interfaceMemberIndex,
			)
			if err != nil {
				if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
					return
				}
				yield(nil, err)
				return
			}

			deviceInterfaceDetailData, deviceInfoData, err := setupDiGetDeviceInterfaceDetailW(
				deviceInfoSet,
				deviceInterfaceData,
			)
			if err != nil {
				yield(nil, err)
				return
			}

			propertyType, statusBuf, err := setupDiGetDevicePropertyW(deviceInfoSet, deviceInfoData, &devpkeyDeviceDevNodeStatus)
			if err != nil {
				yield(nil, err)
				return
			}
			if propertyType != windows.DEVPROP_TYPE_UINT32 {
				yield(nil, errors.New("uint32 was expected"))
				return
			}

			status := *(*uint32)(unsafe.Pointer(&statusBuf[0]))
			if (status&_DN_HAS_PROBLEM) == _DN_HAS_PROBLEM ||
				(status&_DN_STARTED) != _DN_STARTED ||
				(status&_DN_DRIVER_LOADED) != _DN_DRIVER_LOADED {
				continue
			}

			devicePath := windows.UTF16PtrToString(&deviceInterfaceDetailData.DevicePath[0])
			deviceInfo, err := getDeviceInfo(devicePath)
			if err != nil {
				yield(nil, err)
				return
			}
			deviceInfo.InterfaceNbr = int(interfaceMemberIndex)

			instanceID, err := setupDiGetDeviceInstanceIdW(deviceInfoSet, deviceInfoData)
			if err != nil {
				yield(nil, err)
				return
			}
			deviceInfo.InstanceID = instanceID

			propertyType, parentBuf, err := setupDiGetDevicePropertyW(deviceInfoSet, deviceInfoData, &devpkeyDeviceParent)
			if err != nil {
				yield(nil, err)
				return
			}
			if propertyType != windows.DEVPROP_TYPE_STRING {
				yield(nil, errors.New("string was expected"))
				return
			}
			u16ParentBuf := unsafe.Slice((*uint16)(unsafe.Pointer(&parentBuf[0])), len(parentBuf)/2)
			deviceInfo.ParentDeviceID = strings.Clone(windows.UTF16ToString(u16ParentBuf))

			if !opts.match(deviceInfo) {
				continue
			}

			if !yield(deviceInfo, nil) {
				return
			}
		}
	}
}

type Option func(*Device)

func WithReadTimeout(timeout time.Duration) Option {
	return func(device *Device) {
		device.readTimeout = uint32(timeout.Milliseconds())
	}
}

func OpenPath(path string, opts ...Option) (*Device, error) {
	d := &Device{
		readTimeout: windows.INFINITE,
	}
	for _, opt := range opts {
		opt(d)
	}

	devicePathPtr := windows.StringToUTF16Ptr(path)

	hFile, err := windows.CreateFile(
		devicePathPtr,
		windows.GENERIC_WRITE|windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = windows.Close(hFile)
		}
	}()

	preparsedData, err := getPreparsedData(hFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = freePreparsedData(preparsedData)
	}()

	caps, err := getCaps(preparsedData)
	if err != nil {
		return nil, err
	}
	d.inputReportByteLength = caps.InputReportByteLength
	d.outputReportByteLength = caps.OutputReportByteLength
	d.featureReportByteLength = caps.FeatureReportByteLength
	d.hFile = hFile
	closeOnError = false

	return d, nil
}

func (d *Device) Read(ctx context.Context, p []byte) (n int, err error) {
	d.readMu.Lock()

	if err := ctx.Err(); err != nil {
		d.readMu.Unlock()

		return 0, err
	}

	hEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		d.readMu.Unlock()
		return 0, err
	}

	overlapped := &windows.Overlapped{
		HEvent: hEvent,
	}

	result := make(chan ioResult, 1)
	go func() {
		defer d.readMu.Unlock()
		defer func() { _ = windows.Close(hEvent) }()
		result <- d.read(overlapped)
	}()

	select {
	case <-ctx.Done():
		_ = windowsCancelIoEx(d.hFile, overlapped)

		return 0, ctx.Err()

	case r := <-result:
		if r.err != nil {
			return 0, r.err
		}

		return copy(p, r.data), nil
	}
}

func (d *Device) read(overlapped *windows.Overlapped) ioResult {
	buf := make([]byte, d.inputReportByteLength)
	var done uint32
	if err := windowsReadFile(d.hFile, buf, &done, overlapped); err != nil {
		if !errors.Is(err, windows.ERROR_IO_PENDING) {
			return ioResult{err: err}
		}
	}

	event, err := windows.WaitForSingleObject(overlapped.HEvent, d.readTimeout)
	if err != nil {
		return ioResult{err: err}
	}
	if event != windows.WAIT_OBJECT_0 {
		_ = windowsCancelIoEx(d.hFile, overlapped)
		_ = windowsGetOverlappedResult(d.hFile, overlapped, &done, true)
		return ioResult{err: fmt.Errorf("unexpected event: %d", event)}
	}

	if err := windowsGetOverlappedResult(d.hFile, overlapped, &done, true); err != nil {
		return ioResult{err: err}
	}

	if done == 0 {
		return ioResult{err: errors.New("no data received")}
	}

	// Remove report ID
	if buf[0] == 0 {
		buf = buf[1:]
	}

	return ioResult{n: len(buf), data: buf}
}

func (d *Device) Write(ctx context.Context, p []byte) (n int, err error) {
	buf := make([]byte, d.outputReportByteLength)
	copy(buf, p)

	d.writeMu.Lock()

	if err := ctx.Err(); err != nil {
		d.writeMu.Unlock()

		return 0, err
	}

	hEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		d.writeMu.Unlock()
		return 0, err
	}
	overlapped := &windows.Overlapped{HEvent: hEvent}

	result := make(chan ioResult, 1)
	go func() {
		defer d.writeMu.Unlock()
		defer func() { _ = windows.Close(hEvent) }()
		result <- d.write(buf, overlapped)
	}()

	select {
	case <-ctx.Done():
		_ = windowsCancelIoEx(d.hFile, overlapped)

		return 0, ctx.Err()

	case r := <-result:
		return r.n, r.err
	}
}

func (d *Device) write(buf []byte, overlapped *windows.Overlapped) ioResult {
	var done uint32
	if err := windowsWriteFile(d.hFile, buf, &done, overlapped); err != nil {
		if !errors.Is(err, windows.ERROR_IO_PENDING) {
			return ioResult{err: err}
		}
	}

	if err := windowsGetOverlappedResult(d.hFile, overlapped, &done, true); err != nil {
		return ioResult{err: err}
	}

	if done != uint32(len(buf)) {
		return ioResult{err: fmt.Errorf("expected %d bytes, got %d", len(buf), done)}
	}

	return ioResult{n: len(buf)}
}

func (d *Device) SendFeatureReport(report []byte) error {
	buffer := make([]byte, d.featureReportByteLength)
	copy(buffer, report)

	r1, _, err := procHidD_SetFeature.Call(
		uintptr(d.hFile),
		uintptr(unsafe.Pointer(unsafe.SliceData(buffer))),
		uintptr(len(buffer)),
	)
	if r1 == 0 {
		return err
	}

	return nil
}

func (d *Device) GetFeatureReport(report []byte) (int, error) {
	buffer := make([]byte, d.featureReportByteLength)
	buffer[0] = report[0]

	r1, _, err := procHidD_GetFeature.Call(
		uintptr(d.hFile),
		uintptr(unsafe.Pointer(unsafe.SliceData(buffer))),
		uintptr(len(buffer)),
	)
	if r1 == 0 {
		return 0, err
	}

	return copy(report, buffer), nil
}

func (d *Device) Close() error {
	d.closeOnce.Do(func() {
		if err := windowsCancelIoEx(d.hFile, nil); err != nil && !errors.Is(err, windows.ERROR_NOT_FOUND) {
			d.closeErr = err
		}

		if err := windows.Close(d.hFile); err != nil && d.closeErr == nil {
			d.closeErr = err
		}
	})

	return d.closeErr
}
