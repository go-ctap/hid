package hid

import (
	"errors"
	"fmt"
	"strings"
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
	cmCallback                   = syscall.NewCallback(cmNotificationCallback)

	cmReceivers   sync.Map
	cmReceiverSeq atomic.Uint64
)

func cmRegisterNotification(
	pFilter *_CM_NOTIFY_FILTER,
	pContext uintptr,
	pCallback uintptr,
) (windows.Handle, error) {
	if err := procCMRegisterNotification.Find(); err != nil {
		return 0, fmt.Errorf("CM_Register_Notification failed: %w", err)
	}

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
	if err := procCMUnregisterNotification.Find(); err != nil {
		return fmt.Errorf("CM_Unregister_Notification failed: %w", err)
	}

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
	events *deviceEventQueue

	mu            sync.Mutex
	closed        bool
	initializing  bool
	startupEvents []DeviceEvent
	devices       map[string]*DeviceInfo
	once          sync.Once
	closeErr      error

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
	return er.onPathAction(action, path)
}

func (er *cmEventReceiver) onPathAction(action _CM_NOTIFY_ACTION, path string) uintptr {
	er.mu.Lock()
	defer er.mu.Unlock()
	if er.closed {
		return 0
	}

	typ := DeviceEventConnected
	devInfo := &DeviceInfo{Path: path}
	var devErr error
	if action == _CM_NOTIFY_ACTION_DEVICEINTERFACEREMOVAL {
		typ = DeviceEventDisconnected
		if cached := er.devices[cmDevicePathKey(path)]; cached != nil {
			devInfo = cloneCMDeviceInfo(cached)
		}
	} else {
		info, err := getDeviceInfo(path)
		if err == nil {
			devInfo = info
		} else {
			devErr = err
		}
	}

	event := DeviceEvent{
		Type:       typ,
		DeviceInfo: devInfo,
		Err:        devErr,
	}
	if er.initializing {
		er.startupEvents = append(er.startupEvents, event)
		return 0
	}

	key := cmDevicePathKey(path)
	switch typ {
	case DeviceEventConnected:
		er.devices[key] = cloneCMDeviceInfo(devInfo)
	case DeviceEventDisconnected:
		delete(er.devices, key)
	}
	er.events.Send(event)

	return 0
}

func cmNotificationCallback(
	hNotify uintptr,
	context uintptr,
	action _CM_NOTIFY_ACTION,
	eventData unsafe.Pointer,
	eventDataSize uintptr,
) uintptr {
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
		(*_CM_NOTIFY_EVENT_DATA)(eventData),
		eventDataSize,
	)
}

func (er *cmEventReceiver) Listen() <-chan DeviceEvent {
	return er.events.Listen()
}

func (er *cmEventReceiver) Close() error {
	er.once.Do(func() {
		er.mu.Lock()
		er.closed = true
		er.mu.Unlock()

		if er.notify != 0 {
			er.closeErr = cmUnregisterNotification(er.notify)
		}
		er.events.Close()
		cmReceivers.Delete(er.cbID)
	})

	return er.closeErr
}

func cmDevicePathKey(path string) string {
	return strings.ToLower(path)
}

func cloneCMDeviceInfo(info *DeviceInfo) *DeviceInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	return &cloned
}

// reconcileCMStartupEvents applies notifications received while Enumerate was
// running over its result. Notifications win even when SetupAPI yields a stale
// device after CM already reported its removal.
func reconcileCMStartupEvents(snapshot []*DeviceInfo, changes []DeviceEvent) []DeviceEvent {
	state := make(map[string]DeviceEvent, len(snapshot)+len(changes))
	order := make([]string, 0, len(snapshot)+len(changes))
	ordered := make(map[string]struct{}, len(snapshot)+len(changes))

	put := func(event DeviceEvent) {
		if event.DeviceInfo == nil || event.DeviceInfo.Path == "" {
			return
		}

		key := cmDevicePathKey(event.DeviceInfo.Path)
		if _, ok := ordered[key]; !ok {
			ordered[key] = struct{}{}
			order = append(order, key)
		}
		state[key] = event
	}

	for _, info := range snapshot {
		put(DeviceEvent{
			Type:       DeviceEventConnected,
			DeviceInfo: info,
		})
	}

	for _, event := range changes {
		if event.DeviceInfo == nil || event.DeviceInfo.Path == "" {
			continue
		}

		key := cmDevicePathKey(event.DeviceInfo.Path)
		switch event.Type {
		case DeviceEventConnected:
			put(event)
		case DeviceEventDisconnected:
			delete(state, key)
		}
	}

	events := make([]DeviceEvent, 0, len(state))
	for _, key := range order {
		if event, ok := state[key]; ok {
			events = append(events, event)
		}
	}
	return events
}

func (er *cmEventReceiver) publishStartup(snapshot []*DeviceInfo) {
	er.mu.Lock()
	defer er.mu.Unlock()
	if er.closed {
		return
	}

	events := reconcileCMStartupEvents(snapshot, er.startupEvents)
	for _, event := range events {
		er.devices[cmDevicePathKey(event.DeviceInfo.Path)] = cloneCMDeviceInfo(event.DeviceInfo)
		er.events.Send(event)
	}
	er.startupEvents = nil
	er.initializing = false
}

// Events publishes a connected event for every currently enumerated HID device,
// followed by live connection and removal events.
func Events() (EventReceiver, error) {
	hidGuid, err := getHidGuid()
	if err != nil {
		return nil, err
	}

	receiver := &cmEventReceiver{
		events:       newDeviceEventQueue(),
		initializing: true,
		devices:      make(map[string]*DeviceInfo),
	}

	cbID := uintptr(cmReceiverSeq.Add(1))
	receiver.cbID = cbID
	cmReceivers.Store(cbID, receiver)

	filter := &_CM_NOTIFY_FILTER{
		CbSize:     uint32(unsafe.Sizeof(_CM_NOTIFY_FILTER{})),
		FilterType: _CM_NOTIFY_FILTER_TYPE_DEVICEINTERFACE,
	}
	*(*windows.GUID)(unsafe.Pointer(&filter.U[0])) = *hidGuid

	notify, err := cmRegisterNotification(filter, cbID, cmCallback)
	if err != nil {
		_ = receiver.Close()
		return nil, err
	}

	receiver.notify = notify

	var snapshot []*DeviceInfo
	for info, enumerateErr := range Enumerate() {
		if enumerateErr != nil {
			closeErr := receiver.Close()
			return nil, errors.Join(
				fmt.Errorf("enumerate initial HID snapshot: %w", enumerateErr),
				closeErr,
			)
		}
		if info != nil {
			snapshot = append(snapshot, info)
		}
	}
	receiver.publishStartup(snapshot)

	return receiver, nil
}
