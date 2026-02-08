//go:build ignore

package hid

/*
#include <windows.h>
#include <setupapi.h>
#include <hidsdi.h>
#include <hidclass.h>
#include <hidpi.h>
*/
import "C"

type (
	_GUID                              C.GUID
	_TCHAR                             C.TCHAR
	_SP_DEVICE_INTERFACE_DATA          C.SP_DEVICE_INTERFACE_DATA
	_SP_DEVINFO_DATA                   C.SP_DEVINFO_DATA
	_SP_DEVICE_INTERFACE_DETAIL_DATA_W C.SP_DEVICE_INTERFACE_DETAIL_DATA_W
	_HIDD_ATTRIBUTES                   C.HIDD_ATTRIBUTES
	_PHIDP_PREPARSED_DATA              uintptr
	_HIDP_CAPS                         C.HIDP_CAPS
)
