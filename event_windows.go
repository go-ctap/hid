package hid

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type (
	_CM_NOTIFY_FILTER_TYPE uint32
	_CM_NOTIFY_ACTION      uint32
)

const (
	_CM_NOTIFY_FILTER_TYPE_DEVICEINTERFACE   _CM_NOTIFY_FILTER_TYPE = 0x00000000
	_CM_NOTIFY_ACTION_DEVICEINTERFACEARRIVAL _CM_NOTIFY_ACTION      = 0x00000000
	_CM_NOTIFY_ACTION_DEVICEINTERFACEREMOVAL _CM_NOTIFY_ACTION      = 0x00000001
	_MAX_DEVICE_ID_LEN                                              = 200
)

type _CM_NOTIFY_FILTER struct {
	CbSize     uint32
	Flags      uint32
	FilterType _CM_NOTIFY_FILTER_TYPE
	Reserved   uint32
	// Union from CM_NOTIFY_FILTER; largest member is WCHAR[MAX_DEVICE_ID_LEN].
	U [2 * _MAX_DEVICE_ID_LEN]byte
}

type _CM_NOTIFY_EVENT_DATA struct {
	FilterType   _CM_NOTIFY_FILTER_TYPE
	Reserved     uint32
	ClassGuid    windows.GUID
	SymbolicLink [1]uint16
}

var (
	modCfgMgr32                  = windows.NewLazySystemDLL("CfgMgr32.dll")
	procCMRegisterNotification   = modCfgMgr32.NewProc("CM_Register_Notification")
	procCMUnregisterNotification = modCfgMgr32.NewProc("CM_Unregister_Notification")

	cmReceivers   sync.Map
	cmReceiverSeq atomic.Uint64
)

func cmRegisterNotification(
	pFilter *_CM_NOTIFY_FILTER,
	pContext uintptr,
	pCallback uintptr,
) (windows.Handle, error) {
	var pNotifyContext windows.Handle
	r1, _, err := procCMRegisterNotification.Call(
		uintptr(unsafe.Pointer(pFilter)),
		pContext,
		pCallback,
		uintptr(unsafe.Pointer(&pNotifyContext)),
	)
	if !errors.Is(windows.CONFIGRET(r1), windows.CR_SUCCESS) {
		if !errors.Is(err, windows.ERROR_SUCCESS) {
			return 0, err
		}
		return 0, fmt.Errorf("CM_Register_Notification failed: CONFIGRET=0x%x", uint32(r1))
	}

	return pNotifyContext, nil
}

func cmUnregisterNotification(pNotifyContext windows.Handle) error {
	r1, _, err := procCMUnregisterNotification.Call(uintptr(pNotifyContext))
	if !errors.Is(windows.CONFIGRET(r1), windows.CR_SUCCESS) {
		if !errors.Is(err, windows.ERROR_SUCCESS) {
			return err
		}
		return fmt.Errorf("CM_Unregister_Notification failed: CONFIGRET=0x%x", uint32(r1))
	}

	return nil
}

func cmSymbolicLinkFromEventData(data *_CM_NOTIFY_EVENT_DATA, eventDataSize uintptr) string {
	if data == nil {
		return ""
	}

	const wcharSize = uintptr(2)
	offset := unsafe.Offsetof(_CM_NOTIFY_EVENT_DATA{}.SymbolicLink)
	if eventDataSize <= offset {
		return ""
	}

	n := int((eventDataSize - offset) / wcharSize)
	if n <= 0 {
		return ""
	}

	ptr := (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(data)) + offset))
	return windows.UTF16ToString(unsafe.Slice(ptr, n))
}

type cmEventReceiver struct {
	notify windows.Handle
	events chan DeviceEvent

	mu     sync.RWMutex
	closed bool
	once   sync.Once

	cbID uintptr
}

func (er *cmEventReceiver) onAction(action _CM_NOTIFY_ACTION, data *_CM_NOTIFY_EVENT_DATA, eventDataSize uintptr) uintptr {
	if action != _CM_NOTIFY_ACTION_DEVICEINTERFACEARRIVAL && action != _CM_NOTIFY_ACTION_DEVICEINTERFACEREMOVAL {
		return 0
	}
	if data == nil || data.FilterType != _CM_NOTIFY_FILTER_TYPE_DEVICEINTERFACE {
		return 0
	}

	path := cmSymbolicLinkFromEventData(data, eventDataSize)
	if path == "" {
		return 0
	}

	typ := DeviceEventConnected
	devInfo := &DeviceInfo{Path: path}
	var devErr error
	if action == _CM_NOTIFY_ACTION_DEVICEINTERFACEREMOVAL {
		typ = DeviceEventDisconnected
	} else {
		info, err := getDeviceInfo(path)
		if err == nil {
			devInfo = info
		} else {
			devErr = err
		}
	}

	er.mu.RLock()
	defer er.mu.RUnlock()
	if er.closed {
		return 0
	}

	select {
	case er.events <- DeviceEvent{
		Type:       typ,
		DeviceInfo: devInfo,
		Err:        devErr,
	}:
	default:
	}

	return 0
}

func (er *cmEventReceiver) Listen() <-chan DeviceEvent {
	return er.events
}

func (er *cmEventReceiver) Close() error {
	var unregisterErr error

	er.once.Do(func() {
		er.mu.Lock()
		er.closed = true
		close(er.events)
		er.mu.Unlock()

		if er.notify != 0 {
			unregisterErr = cmUnregisterNotification(er.notify)
		}
		cmReceivers.Delete(er.cbID)
	})

	return unregisterErr
}

func Events() (EventReceiver, error) {
	hidGuid, err := getHidGuid()
	if err != nil {
		return nil, err
	}

	receiver := &cmEventReceiver{
		events: make(chan DeviceEvent, 10),
	}

	cbID := uintptr(cmReceiverSeq.Add(1))
	receiver.cbID = cbID
	cmReceivers.Store(cbID, receiver)

	callback := syscall.NewCallback(func(hNotify uintptr, context uintptr, action _CM_NOTIFY_ACTION, eventData uintptr, eventDataSize uintptr) uintptr {
		v, ok := cmReceivers.Load(context)
		if !ok {
			return 0
		}

		er, ok := v.(*cmEventReceiver)
		if !ok {
			return 0
		}

		return er.onAction(
			action,
			(*_CM_NOTIFY_EVENT_DATA)(unsafe.Pointer(eventData)),
			eventDataSize,
		)
	})

	filter := &_CM_NOTIFY_FILTER{
		CbSize:     uint32(unsafe.Sizeof(_CM_NOTIFY_FILTER{})),
		FilterType: _CM_NOTIFY_FILTER_TYPE_DEVICEINTERFACE,
	}
	*(*windows.GUID)(unsafe.Pointer(&filter.U[0])) = *hidGuid

	notify, err := cmRegisterNotification(filter, cbID, callback)
	if err != nil {
		cmReceivers.Delete(cbID)
		return nil, err
	}

	receiver.notify = notify
	return receiver, nil
}
