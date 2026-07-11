package hid

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	darwinDeviceMatchingCallbackPtr = purego.NewCallback(darwinDeviceMatchingCallback)
	darwinDeviceRemovalCallbackPtr  = purego.NewCallback(darwinDeviceRemovalCallback)

	darwinEventReceivers sync.Map
	darwinEventSeq       atomic.Uint64
)

type darwinEventReceiver struct {
	events  *deviceEventQueue
	ready   chan error
	stopped chan struct{}

	mu      sync.Mutex
	closed  bool
	runLoop uintptr
	devices map[ioHIDDeviceRef]*DeviceInfo

	closeOnce sync.Once
	cbID      uintptr
}

func (er *darwinEventReceiver) Listen() <-chan DeviceEvent {
	return er.events.Listen()
}

func (er *darwinEventReceiver) Close() error {
	er.closeOnce.Do(func() {
		er.mu.Lock()
		er.closed = true
		runLoop := er.runLoop
		er.mu.Unlock()

		if runLoop != 0 {
			cfRunLoopStop(runLoop)
		}
		<-er.stopped
	})

	return nil
}

func (er *darwinEventReceiver) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(er.stopped)
	defer er.events.Close()
	defer darwinEventReceivers.Delete(er.cbID)

	manager := ioHIDManagerCreate(kCFAllocatorDefault, kIOHIDManagerOptionNone)
	if manager == 0 {
		er.ready <- errors.New("IOHIDManagerCreate failed")
		return
	}
	defer cfRelease(cfTypeRef(manager))

	runLoop := cfRunLoopGetCurrent()
	er.mu.Lock()
	er.runLoop = runLoop
	er.mu.Unlock()

	// nil means all HID devices. Opening the manager is deliberately omitted:
	// enumeration callbacks and properties do not require device access, while
	// IOHIDManagerOpen can fail for protected HID devices for an unprivileged user.
	ioHIDManagerSetDeviceMatching(manager, 0)
	ioHIDManagerRegisterDeviceMatchingCallback(manager, darwinDeviceMatchingCallbackPtr, er.cbID)
	ioHIDManagerRegisterDeviceRemovalCallback(manager, darwinDeviceRemovalCallbackPtr, er.cbID)
	ioHIDManagerScheduleWithRunLoop(manager, runLoop, cfRunLoopDefaultMode)
	er.publishCurrentDevices(manager)

	er.ready <- nil
	for {
		er.mu.Lock()
		closed := er.closed
		er.mu.Unlock()
		if closed {
			break
		}

		// The finite timeout makes shutdown bounded even if Close happens just
		// before the first run-loop invocation and CFRunLoopStop has no waiter.
		if cfRunLoopRunInMode(cfRunLoopDefaultMode, 1, false) == kCFRunLoopRunFinished {
			// A scheduled IOHIDManager normally keeps a source installed. Avoid
			// spinning if Core Foundation transiently reports an empty run loop.
			time.Sleep(10 * time.Millisecond)
		}
	}

	ioHIDManagerUnscheduleFromRunLoop(manager, runLoop, cfRunLoopDefaultMode)
	ioHIDManagerRegisterDeviceMatchingCallback(manager, 0, 0)
	ioHIDManagerRegisterDeviceRemovalCallback(manager, 0, 0)
	er.mu.Lock()
	er.runLoop = 0
	er.mu.Unlock()
}

// publishCurrentDevices explicitly queues the initial snapshot before Events
// returns. The matching callbacks delivered by the run loop are deduplicated by
// deviceConnected, while changes after this copy remain queued as live callbacks.
func (er *darwinEventReceiver) publishCurrentDevices(manager ioHIDManagerRef) {
	deviceSet := ioHIDManagerCopyDevices(manager)
	if deviceSet == 0 {
		return
	}
	defer cfRelease(cfTypeRef(deviceSet))

	count := int(cfSetGetCount(deviceSet))
	if count <= 0 {
		return
	}

	devices := make([]uintptr, count)
	cfSetGetValues(deviceSet, uintptr(unsafe.Pointer(unsafe.SliceData(devices))))
	for _, device := range devices {
		if device != 0 {
			er.deviceConnected(kIOReturnSuccess, ioHIDDeviceRef(device))
		}
	}
}

func (er *darwinEventReceiver) deviceConnected(result ioReturn, device ioHIDDeviceRef) {
	info, infoErr := getDeviceInfo(device)
	if info == nil {
		info = &DeviceInfo{}
	}
	eventErr := errors.Join(callbackResultError("IOHIDManager device matching", result), infoErr)

	er.mu.Lock()
	if er.closed {
		er.mu.Unlock()
		return
	}
	if _, exists := er.devices[device]; exists {
		er.mu.Unlock()
		return
	}
	er.devices[device] = cloneDeviceInfo(info)
	er.mu.Unlock()

	er.events.Send(DeviceEvent{
		Type:       DeviceEventConnected,
		DeviceInfo: cloneDeviceInfo(info),
		Err:        eventErr,
	})
}

func (er *darwinEventReceiver) deviceDisconnected(result ioReturn, device ioHIDDeviceRef) {
	er.mu.Lock()
	if er.closed {
		er.mu.Unlock()
		return
	}
	info := er.devices[device]
	delete(er.devices, device)
	er.mu.Unlock()

	var infoErr error
	if info == nil {
		info, infoErr = getDeviceInfo(device)
	}
	if info == nil {
		info = &DeviceInfo{}
	}

	er.events.Send(DeviceEvent{
		Type:       DeviceEventDisconnected,
		DeviceInfo: cloneDeviceInfo(info),
		Err:        errors.Join(callbackResultError("IOHIDManager device removal", result), infoErr),
	})
}

func callbackResultError(operation string, result ioReturn) error {
	if result == kIOReturnSuccess {
		return nil
	}
	return fmt.Errorf("%s callback failed: 0x%08x", operation, uint32(result))
}

func cloneDeviceInfo(info *DeviceInfo) *DeviceInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	return &cloned
}

func darwinDeviceMatchingCallback(context uintptr, result ioReturn, _ uintptr, device ioHIDDeviceRef) {
	value, ok := darwinEventReceivers.Load(context)
	if !ok {
		return
	}
	receiver, ok := value.(*darwinEventReceiver)
	if !ok {
		return
	}
	receiver.deviceConnected(result, device)
}

func darwinDeviceRemovalCallback(context uintptr, result ioReturn, _ uintptr, device ioHIDDeviceRef) {
	value, ok := darwinEventReceivers.Load(context)
	if !ok {
		return
	}
	receiver, ok := value.(*darwinEventReceiver)
	if !ok {
		return
	}
	receiver.deviceDisconnected(result, device)
}

// Events publishes a connected event for every currently enumerated HID device,
// followed by live connection and removal events.
func Events() (EventReceiver, error) {
	receiver := &darwinEventReceiver{
		events:  newDeviceEventQueue(),
		ready:   make(chan error, 1),
		stopped: make(chan struct{}),
		devices: make(map[ioHIDDeviceRef]*DeviceInfo),
	}

	receiver.cbID = uintptr(darwinEventSeq.Add(1))
	darwinEventReceivers.Store(receiver.cbID, receiver)
	go receiver.run()

	if err := <-receiver.ready; err != nil {
		<-receiver.stopped
		return nil, err
	}
	return receiver, nil
}
