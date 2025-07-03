//go:generate powershell -Command "go tool cgo -godefs types_hid_windows.go | Set-Content -Path ztypes_hid_windows.go -Encoding UTF8"
package hid

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/unicode"
)

var (
	modHidsdi                             = windows.NewLazySystemDLL("hid.dll")
	procHidD_GetHidGuid                   = modHidsdi.NewProc("HidD_GetHidGuid")
	procHidD_GetAttributes                = modHidsdi.NewProc("HidD_GetAttributes")
	procHidD_GetManufacturerString        = modHidsdi.NewProc("HidD_GetManufacturerString")
	procHidD_GetProductString             = modHidsdi.NewProc("HidD_GetProductString")
	procHidD_GetSerialNumberString        = modHidsdi.NewProc("HidD_GetSerialNumberString")
	procHidD_GetPreparsedData             = modHidsdi.NewProc("HidD_GetPreparsedData")
	procHidD_FreePreparsedData            = modHidsdi.NewProc("HidD_FreePreparsedData")
	procHidP_GetCaps                      = modHidsdi.NewProc("HidP_GetCaps")
	modSetupapi                           = windows.NewLazySystemDLL("setupapi.dll")
	procSetupDiGetClassDevsW              = modSetupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInfo             = modSetupapi.NewProc("SetupDiEnumDeviceInfo")
	procSetupDiEnumDeviceInterfaces       = modSetupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetailW  = modSetupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiGetDeviceRegistryPropertyW = modSetupapi.NewProc("SetupDiGetDeviceRegistryPropertyW")
)

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

	return buf, nil
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

	return buf, nil
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
	flags uint32,
) (_HDEVINFO, error) {
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
		return nil, err
	}

	return _HDEVINFO(unsafe.Pointer(r1)), nil
}

func setupDiEnumDeviceInfo(
	hdevinfo _HDEVINFO,
	memberIndex uint32,
) (*_SP_DEVINFO_DATA, error) {
	var deviceInfoData _SP_DEVINFO_DATA
	deviceInfoData.CbSize = uint32(unsafe.Sizeof(deviceInfoData))

	r1, _, err := procSetupDiEnumDeviceInfo.Call(
		uintptr(unsafe.Pointer(hdevinfo)),
		uintptr(memberIndex),
		uintptr(unsafe.Pointer(&deviceInfoData)),
	)
	if r1 == 0 {
		return nil, err
	}

	return &deviceInfoData, nil
}

func setupDiEnumDeviceInterfaces(
	deviceInfoSet _HDEVINFO,
	devInfoData *_SP_DEVINFO_DATA,
	interfaceClassGuid *windows.GUID,
	memberIndex uint32,
) (*_SP_DEVICE_INTERFACE_DATA, error) {
	var deviceInterfaceData _SP_DEVICE_INTERFACE_DATA
	deviceInterfaceData.CbSize = uint32(unsafe.Sizeof(deviceInterfaceData))

	r1, _, err := procSetupDiEnumDeviceInterfaces.Call(
		uintptr(unsafe.Pointer(deviceInfoSet)),
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

func setupDiGetDeviceInterfaceDetailW(
	deviceInfoSet _HDEVINFO,
	deviceInterfaceData *_SP_DEVICE_INTERFACE_DATA,
) (
	deviceInterfaceDetailData *_SP_DEVICE_INTERFACE_DETAIL_DATA_W,
	deviceInfoData *_SP_DEVINFO_DATA,
	err error,
) {
	var requiredSize uint32
	r1, _, err := procSetupDiGetDeviceInterfaceDetailW.Call(
		uintptr(unsafe.Pointer(deviceInfoSet)),
		uintptr(unsafe.Pointer(deviceInterfaceData)),
		0,
		0,
		uintptr(unsafe.Pointer(&requiredSize)),
		uintptr(unsafe.Pointer(deviceInfoData)),
	)
	if r1 == 0 && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return nil, nil, err
	}

	deviceInterfaceDetailData = new(_SP_DEVICE_INTERFACE_DETAIL_DATA_W)
	deviceInterfaceDetailData.CbSize = uint32(unsafe.Sizeof(*deviceInterfaceDetailData))
	deviceInfoData = new(_SP_DEVINFO_DATA)
	deviceInfoData.CbSize = uint32(unsafe.Sizeof(*deviceInfoData))

	r1, _, err = procSetupDiGetDeviceInterfaceDetailW.Call(
		uintptr(unsafe.Pointer(deviceInfoSet)),
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

func setupDiGetDeviceRegistryPropertyW(
	deviceInfoSet _HDEVINFO,
	deviceInfoData *_SP_DEVINFO_DATA,
	property uint32,
) (
	propertyRegDataType uint32,
	propertyBuffer []byte,
	err error,
) {
	var requiredSize uint32
	r1, _, err := procSetupDiGetDeviceRegistryPropertyW.Call(
		uintptr(unsafe.Pointer(deviceInfoSet)),
		uintptr(unsafe.Pointer(deviceInfoData)),
		uintptr(property),
		uintptr(unsafe.Pointer(&propertyRegDataType)),
		0,
		0,
		uintptr(unsafe.Pointer(&requiredSize)),
	)
	if r1 == 0 && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return 0, nil, err
	}

	propertyBuffer = make([]byte, requiredSize)

	r1, _, err = procSetupDiGetDeviceRegistryPropertyW.Call(
		uintptr(unsafe.Pointer(deviceInfoSet)),
		uintptr(unsafe.Pointer(deviceInfoData)),
		uintptr(property),
		uintptr(unsafe.Pointer(&propertyRegDataType)),
		uintptr(unsafe.Pointer(&propertyBuffer[0])),
		uintptr(requiredSize),
		uintptr(unsafe.Pointer(&requiredSize)),
	)
	if r1 == 0 {
		return 0, nil, err
	}

	return propertyRegDataType, propertyBuffer, nil
}

func Enumerate() iter.Seq2[*DeviceInfo, error] {
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
			_DIGCF_PRESENT|_DIGCF_DEVICEINTERFACE,
		)
		if err != nil {
			yield(nil, err)
			return
		}

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

			deviceInterfaceDetailData, _, err := setupDiGetDeviceInterfaceDetailW(
				deviceInfoSet,
				deviceInterfaceData,
			)
			if err != nil {
				yield(nil, err)
				return
			}

			devicePath := windows.UTF16PtrToString(&deviceInterfaceDetailData.DevicePath[0])
			decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
			hasDriver := false
			for deviceMemberIndex := uint32(0); ; deviceMemberIndex++ {
				deviceInfoData, err := setupDiEnumDeviceInfo(deviceInfoSet, deviceMemberIndex)
				if err != nil {
					if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
						return
					}
					yield(nil, err)
					return
				}

				_, classPropertyBuf, err := setupDiGetDeviceRegistryPropertyW(deviceInfoSet, deviceInfoData, _SPDRP_CLASS)
				if err != nil {
					yield(nil, err)
					return
				}

				classNameRaw, err := decoder.Bytes(bytes.TrimSuffix(classPropertyBuf, []byte{0, 0}))
				if err != nil {
					yield(nil, err)
					return
				}

				if string(classNameRaw) == "HIDClass" {
					_, driverPropertyBuf, err := setupDiGetDeviceRegistryPropertyW(deviceInfoSet, deviceInfoData, _SPDRP_DRIVER)
					if err != nil {
						yield(nil, err)
						return
					}

					driverNameRaw, err := decoder.Bytes(bytes.TrimSuffix(driverPropertyBuf, []byte{0, 0}))
					if err != nil {
						yield(nil, err)
						return
					}

					hasDriver = string(driverNameRaw) != ""
					break
				}
			}

			if !hasDriver {
				yield(nil, nil)
				return
			}

			devicePathPtr := windows.StringToUTF16Ptr(devicePath)

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
				yield(nil, err)
				return
			}

			deviceInfo := &DeviceInfo{
				Path:         devicePath,
				InterfaceNbr: int(interfaceMemberIndex),
			}

			attrs, err := getAttributes(hFile)
			if err != nil {
				yield(nil, err)
				return
			}
			deviceInfo.VendorID = attrs.VendorID
			deviceInfo.ProductID = attrs.ProductID
			deviceInfo.ReleaseNbr = attrs.VersionNumber

			mfrStr, _ := getManufacturerString(hFile)
			if len(mfrStr) > 0 {
				deviceInfo.MfrStr, err = decoder.String(strings.TrimRight(string(mfrStr), string([]byte{0})) + "\u0000")
				if err != nil {
					yield(nil, err)
					return
				}
			}

			productStr, _ := getProductString(hFile)
			if len(mfrStr) > 0 {
				deviceInfo.ProductStr, err = decoder.String(strings.TrimRight(string(productStr), string([]byte{0})) + "\u0000")
				if err != nil {
					yield(nil, err)
					return
				}
			}

			serialNumberStr, _ := getSerialNumberString(hFile)
			if len(serialNumberStr) > 0 {
				deviceInfo.SerialNbr, err = decoder.String(strings.TrimRight(string(serialNumberStr), string([]byte{0})) + "\u0000")
				if err != nil {
					yield(nil, err)
					return
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
				yield(nil, err)
				return
			}

			if !yield(deviceInfo, nil) {
				return
			}
		}
	}
}

func OpenPath(path string) (*Device, error) {
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

	hEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil, err
	}

	return &Device{
		hFile: hFile,
		overlapped: &windows.Overlapped{
			HEvent: hEvent,
		},
		inputReportByteLength:   caps.InputReportByteLength,
		outputReportByteLength:  caps.OutputReportByteLength,
		featureReportByteLength: caps.FeatureReportByteLength,
	}, nil
}

func (d *Device) Read(p []byte) (n int, err error) {
	if err := windows.ResetEvent(d.overlapped.HEvent); err != nil {
		return 0, err
	}

	buf := make([]byte, d.inputReportByteLength)
	if err := windows.ReadFile(d.hFile, buf, nil, d.overlapped); err != nil {
		if !errors.Is(err, windows.ERROR_IO_PENDING) {
			return 0, err
		}
	}

	event, err := windows.WaitForSingleObject(d.overlapped.HEvent, windows.INFINITE)
	if err != nil {
		return 0, err
	}
	if event != windows.WAIT_OBJECT_0 {
		return 0, fmt.Errorf("unexpected event: %d", event)
	}

	var done uint32
	if err := windows.GetOverlappedResult(d.hFile, d.overlapped, &done, true); err != nil {
		return 0, err
	}

	if done == 0 {
		return 0, fmt.Errorf("no data received")
	}

	// Remove report ID
	if buf[0] == 0 {
		buf = buf[1:]
	}

	return copy(p, buf), nil
}

func (d *Device) Write(p []byte) (n int, err error) {
	buf := make([]byte, d.inputReportByteLength)
	copy(buf, p)

	ol := new(windows.Overlapped)
	if err := windows.WriteFile(d.hFile, buf, nil, ol); err != nil {
		if !errors.Is(err, windows.ERROR_IO_PENDING) {
			return 0, err
		}
	}

	var done uint32
	if err := windows.GetOverlappedResult(d.hFile, ol, &done, true); err != nil {
		return 0, err
	}

	if done != uint32(len(buf)) {
		return 0, fmt.Errorf("expected %d bytes, got %d", len(buf), done)
	}

	return len(buf), nil
}

func (d *Device) Close() error {
	if err := windows.Close(d.overlapped.HEvent); err != nil {
		return err
	}

	return windows.Close(d.hFile)
}
