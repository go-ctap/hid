package hid

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestParseLinuxUevent(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		wantOK     bool
		wantType   DeviceEventType
		wantDevice string
	}{
		{
			name:       "add",
			message:    "add@/devices/pci0000:00/hidraw/hidraw0\x00ACTION=add\x00DEVPATH=/devices/pci0000:00/hidraw/hidraw0\x00SUBSYSTEM=hidraw\x00DEVNAME=hidraw0\x00SEQNUM=1\x00",
			wantOK:     true,
			wantType:   DeviceEventConnected,
			wantDevice: "hidraw0",
		},
		{
			name:       "remove",
			message:    "remove@/devices/hidraw/hidraw17\x00ACTION=remove\x00SUBSYSTEM=hidraw\x00DEVNAME=hidraw17",
			wantOK:     true,
			wantType:   DeviceEventDisconnected,
			wantDevice: "hidraw17",
		},
		{
			name:       "header action and devpath fallback",
			message:    "add@/devices/virtual/hidraw/hidraw2\x00DEVPATH=/devices/virtual/hidraw/hidraw2\x00SUBSYSTEM=hidraw\x00",
			wantOK:     true,
			wantType:   DeviceEventConnected,
			wantDevice: "hidraw2",
		},
		{
			name:    "unsupported action",
			message: "change@/devices/hidraw/hidraw0\x00ACTION=change\x00SUBSYSTEM=hidraw\x00DEVNAME=hidraw0\x00",
		},
		{
			name:    "other subsystem",
			message: "add@/devices/input/input0\x00ACTION=add\x00SUBSYSTEM=input\x00DEVNAME=hidraw0\x00",
		},
		{
			name:    "missing action",
			message: "SUBSYSTEM=hidraw\x00DEVNAME=hidraw0\x00",
		},
		{
			name:    "missing device",
			message: "add@/devices/example\x00ACTION=add\x00SUBSYSTEM=hidraw\x00",
		},
		{
			name:    "path traversal device",
			message: "add@/devices/hidraw/hidraw0\x00ACTION=add\x00DEVPATH=/devices/hidraw/hidraw0\x00SUBSYSTEM=hidraw\x00DEVNAME=../hidraw0\x00",
		},
		{
			name:    "dev directory prefix",
			message: "add@/devices/hidraw/hidraw0\x00ACTION=add\x00SUBSYSTEM=hidraw\x00DEVNAME=/dev/hidraw0\x00",
		},
		{
			name:    "non-numeric suffix",
			message: "add@/devices/hidraw/hidrawx\x00ACTION=add\x00SUBSYSTEM=hidraw\x00DEVNAME=hidrawx\x00",
		},
		{
			name:    "truncated device name",
			message: "add@/devices/hidraw/hidraw\x00ACTION=add\x00SUBSYSTEM=hidraw\x00DEVNAME=hidraw",
		},
		{
			name: "empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseLinuxUevent([]byte(test.message))
			if ok != test.wantOK {
				t.Fatalf("parseLinuxUevent ok = %v, want %v; event=%#v", ok, test.wantOK, got)
			}
			if !test.wantOK {
				return
			}
			if got.eventType != test.wantType || got.device != test.wantDevice {
				t.Fatalf("parseLinuxUevent = %#v, want type=%q device=%q", got, test.wantType, test.wantDevice)
			}
		})
	}
}

func FuzzParseLinuxUevent(f *testing.F) {
	f.Add([]byte("add@/devices/hidraw/hidraw0\x00ACTION=add\x00SUBSYSTEM=hidraw\x00DEVNAME=hidraw0\x00"))
	f.Add([]byte("remove@/devices/hidraw/hidraw0\x00SUBSYSTEM=hidraw\x00DEVNAME=hidraw0\x00"))
	f.Add([]byte("\x00\xff\x00DEVNAME=../../hidraw0\x00"))
	f.Fuzz(func(t *testing.T, message []byte) {
		_, _ = parseLinuxUevent(message)
	})
}

func TestIsKernelUeventSender(t *testing.T) {
	tests := []struct {
		name   string
		sender unix.Sockaddr
		want   bool
	}{
		{name: "kernel", sender: &unix.SockaddrNetlink{Pid: 0}, want: true},
		{name: "userspace", sender: &unix.SockaddrNetlink{Pid: 1234}},
		{name: "wrong socket family", sender: &unix.SockaddrUnix{Name: "test"}},
		{name: "nil"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isKernelUeventSender(test.sender); got != test.want {
				t.Fatalf("isKernelUeventSender() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestReconcileLinuxStartupEvents(t *testing.T) {
	metadataErr := errors.New("metadata unavailable")
	snapshot := []DeviceEvent{
		{Type: DeviceEventConnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw0", ProductID: 1}},
		{Type: DeviceEventConnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw1", ProductID: 2}},
		{Type: DeviceEventConnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw2", ProductID: 3}},
		{Type: DeviceEventConnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw0", ProductID: 4}},
	}
	changes := []DeviceEvent{
		{Type: DeviceEventDisconnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw2"}},
		{Type: DeviceEventConnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw3", ProductID: 5}, Err: metadataErr},
		{Type: DeviceEventDisconnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw3"}},
		{Type: DeviceEventDisconnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw1"}},
		{Type: DeviceEventConnected, DeviceInfo: &DeviceInfo{Path: "/dev/hidraw1", ProductID: 6}, Err: metadataErr},
	}

	events := reconcileLinuxStartupEvents(snapshot, changes)
	if len(events) != 2 {
		t.Fatalf("got %d startup events, want 2: %#v", len(events), events)
	}
	if got := events[0]; got.DeviceInfo == nil || got.DeviceInfo.Path != "/dev/hidraw0" || got.DeviceInfo.ProductID != 4 {
		t.Fatalf("first event = %#v, want de-duplicated hidraw0 snapshot", got)
	}
	if got := events[1]; got.DeviceInfo == nil || got.DeviceInfo.Path != "/dev/hidraw1" ||
		got.DeviceInfo.ProductID != 6 || !errors.Is(got.Err, metadataErr) {
		t.Fatalf("second event = %#v, want latest hidraw1 add", got)
	}

	for _, event := range events {
		if event.DeviceInfo.Path == "/dev/hidraw2" || event.DeviceInfo.Path == "/dev/hidraw3" {
			t.Fatalf("removed startup device was retained: %#v", event)
		}
	}
}

func receiveLinuxEvent(t *testing.T, events <-chan DeviceEvent) DeviceEvent {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("event channel closed before the expected event")
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Linux HID event")
		return DeviceEvent{}
	}
}

func TestLinuxRemovalUsesCachedDeviceInfo(t *testing.T) {
	queue := newDeviceEventQueue()
	defer queue.Close()

	cached := &DeviceInfo{
		Path:       "/dev/hidraw7",
		VendorID:   0x1050,
		ProductID:  0x0407,
		UsagePage:  0xf1d0,
		Usage:      1,
		ProductStr: "Security Key",
	}
	receiver := &linuxEventReceiver{
		events: queue,
		devices: map[string]*DeviceInfo{
			cached.Path: cached,
		},
	}

	receiver.onUevent(linuxUevent{eventType: DeviceEventDisconnected, device: "hidraw7"})
	event := receiveLinuxEvent(t, queue.Listen())
	if event.Type != DeviceEventDisconnected {
		t.Fatalf("event type = %q, want %q", event.Type, DeviceEventDisconnected)
	}
	if event.DeviceInfo == nil || event.DeviceInfo.UsagePage != 0xf1d0 ||
		event.DeviceInfo.Usage != 1 || event.DeviceInfo.ProductStr != "Security Key" {
		t.Fatalf("event info = %#v, want cached FIDO metadata", event.DeviceInfo)
	}
	if event.DeviceInfo == cached {
		t.Fatal("event exposes the receiver's cached DeviceInfo pointer")
	}

	// A repeated removal is still a kernel event. With no cache left, it falls
	// back to the stable device path.
	receiver.onUevent(linuxUevent{eventType: DeviceEventDisconnected, device: "hidraw7"})
	event = receiveLinuxEvent(t, queue.Listen())
	if event.DeviceInfo == nil || event.DeviceInfo.Path != "/dev/hidraw7" ||
		event.DeviceInfo.VendorID != 0 || event.DeviceInfo.UsagePage != 0 {
		t.Fatalf("fallback event info = %#v", event.DeviceInfo)
	}
}

func TestLinuxAddPublishesPartialDeviceInfo(t *testing.T) {
	queue := newDeviceEventQueue()
	defer queue.Close()

	metadataErr := errors.New("sysfs is not ready")
	loaded := &DeviceInfo{VendorID: 0x1050, UsagePage: 0xf1d0}
	receiver := &linuxEventReceiver{
		events:  queue,
		devices: make(map[string]*DeviceInfo),
		loadDeviceInfo: func(name string) (*DeviceInfo, error) {
			if name != "hidraw9" {
				t.Fatalf("loader name = %q, want hidraw9", name)
			}
			return loaded, metadataErr
		},
	}

	receiver.onUevent(linuxUevent{eventType: DeviceEventConnected, device: "hidraw9"})
	event := receiveLinuxEvent(t, queue.Listen())
	if event.Type != DeviceEventConnected || event.DeviceInfo == nil {
		t.Fatalf("event = %#v, want connected event with partial info", event)
	}
	if event.DeviceInfo.Path != "/dev/hidraw9" || event.DeviceInfo.VendorID != 0x1050 ||
		event.DeviceInfo.UsagePage != 0xf1d0 || !errors.Is(event.Err, metadataErr) {
		t.Fatalf("partial event = %#v", event)
	}
	if event.DeviceInfo == loaded {
		t.Fatal("event exposes the metadata loader's DeviceInfo pointer")
	}
	cached := receiver.devices["/dev/hidraw9"]
	if cached == nil || cached == event.DeviceInfo || cached.VendorID != 0x1050 {
		t.Fatalf("receiver cache = %#v, want a separate metadata copy", cached)
	}
}

func TestLinuxDuplicateAddIsSuppressed(t *testing.T) {
	queue := newDeviceEventQueue()
	defer queue.Close()

	cached := &DeviceInfo{Path: "/dev/hidraw4", ProductID: 4}
	receiver := &linuxEventReceiver{
		events: queue,
		devices: map[string]*DeviceInfo{
			cached.Path: cached,
		},
		loadDeviceInfo: func(string) (*DeviceInfo, error) {
			t.Fatal("metadata loader was called for a cached duplicate add")
			return nil, nil
		},
	}

	receiver.onUevent(linuxUevent{eventType: DeviceEventConnected, device: "hidraw4"})
	select {
	case event := <-queue.Listen():
		t.Fatalf("duplicate add published an event: %#v", event)
	case <-time.After(100 * time.Millisecond):
	}
	if receiver.devices[cached.Path] != cached {
		t.Fatal("duplicate add replaced the cached device")
	}
}

func TestLinuxTruncatedUeventIsTerminal(t *testing.T) {
	sockets, err := unix.Socketpair(
		unix.AF_UNIX,
		unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(sockets[0])
	defer unix.Close(sockets[1])

	receiver := &linuxEventReceiver{
		socketFD: sockets[0],
	}

	message := []byte("this datagram is deliberately longer than one byte")
	if _, err := unix.Write(sockets[1], message); err != nil {
		t.Fatal(err)
	}
	if err := receiver.drainUevents(make([]byte, 1)); err == nil ||
		!strings.Contains(err.Error(), "message was truncated") {
		t.Fatalf("drainUevents error = %v, want terminal truncation error", err)
	}
}

func TestLinuxEventReceiverConcurrentClose(t *testing.T) {
	socketFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		t.Fatal(err)
	}
	wakeFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		_ = unix.Close(socketFD)
		t.Fatal(err)
	}

	receiver := &linuxEventReceiver{
		events:   newDeviceEventQueue(),
		socketFD: socketFD,
		wakeFD:   wakeFD,
		stopped:  make(chan struct{}),
		devices:  make(map[string]*DeviceInfo),
	}
	go receiver.run()

	const closerCount = 8
	errs := make(chan error, closerCount)
	var closers sync.WaitGroup
	closers.Add(closerCount)
	for range closerCount {
		go func() {
			defer closers.Done()
			errs <- receiver.Close()
		}()
	}

	done := make(chan struct{})
	go func() {
		closers.Wait()
		close(errs)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Close calls did not unblock the Linux event receiver")
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}
	if _, ok := <-receiver.Listen(); ok {
		t.Fatal("Listen remains open after Close")
	}
}

func TestLinuxEventReceiverReportsBackgroundReadFailure(t *testing.T) {
	socketFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		t.Fatal(err)
	}
	wakeFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		_ = unix.Close(socketFD)
		t.Fatal(err)
	}

	receiver := &linuxEventReceiver{
		events:   newDeviceEventQueue(),
		socketFD: socketFD,
		wakeFD:   wakeFD,
		stopped:  make(chan struct{}),
		devices:  make(map[string]*DeviceInfo),
	}
	go receiver.run()
	if err := signalLinuxEventFD(socketFD); err != nil {
		t.Fatal(err)
	}

	select {
	case _, ok := <-receiver.Listen():
		if ok {
			t.Fatal("unexpected event before background failure")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event channel did not close after background read failure")
	}
	if err := receiver.Close(); !errors.Is(err, unix.ENOTSOCK) {
		t.Fatalf("Close error = %v, want ENOTSOCK background failure", err)
	}
}

func linuxIntegrationEventReceiver(t *testing.T) EventReceiver {
	t.Helper()
	receiver, err := Events()
	if err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) ||
			errors.Is(err, unix.EPROTONOSUPPORT) || errors.Is(err, unix.EAFNOSUPPORT) {
			t.Skipf("Linux uevent backend is unavailable in this environment: %v", err)
		}
		t.Fatal(err)
	}
	return receiver
}

func TestEventLifecycle(t *testing.T) {
	receiver := linuxIntegrationEventReceiver(t)
	events := receiver.Listen()

	if err := receiver.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := receiver.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("event channel remains open after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event channel was not closed after Close")
	}
}

func TestEventsAllowMultipleReceivers(t *testing.T) {
	first := linuxIntegrationEventReceiver(t)
	defer first.Close()
	second := linuxIntegrationEventReceiver(t)
	defer second.Close()
}

func TestEventsPublishInitialSnapshot(t *testing.T) {
	receiver := linuxIntegrationEventReceiver(t)
	defer receiver.Close()

	names, err := linuxHIDRawNames()
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	expected := make(map[string]struct{}, len(names))
	for _, name := range names {
		if isLinuxHIDRawName(name) {
			expected[linuxHIDRawPath(name)] = struct{}{}
		}
	}
	initialPaths := make(map[string]struct{}, len(expected))
	for path := range expected {
		initialPaths[path] = struct{}{}
	}

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for len(expected) > 0 {
		select {
		case event, ok := <-receiver.Listen():
			if !ok {
				t.Fatal("event channel closed during initial snapshot")
			}
			if event.Type != DeviceEventConnected || event.DeviceInfo == nil {
				continue
			}
			path := event.DeviceInfo.Path
			if _, isInitial := initialPaths[path]; !isInitial {
				continue
			}
			delete(expected, path)
		case <-deadline.C:
			t.Fatalf("initial snapshot is missing %d device(s): %v", len(expected), expected)
		}
	}
}

func waitForLinuxEvent(t *testing.T, events <-chan DeviceEvent, eventType DeviceEventType, hint string, timeout time.Duration) DeviceEvent {
	t.Helper()
	hint = strings.ToLower(hint)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("event channel closed before expected event")
			}
			if event.Type != eventType || event.DeviceInfo == nil {
				continue
			}
			info := event.DeviceInfo
			haystack := strings.ToLower(strings.Join([]string{
				info.Path,
				info.MfrStr,
				info.ProductStr,
				info.SerialNbr,
			}, " "))
			if hint != "" && !strings.Contains(haystack, hint) {
				continue
			}
			t.Logf("event type=%s path=%q product=%q err=%v", event.Type, info.Path, info.ProductStr, event.Err)
			return event
		case <-deadline.C:
			t.Fatalf("timed out waiting for %s event (hint=%q)", eventType, hint)
		}
	}
}

func TestEventManualDisconnectConnect(t *testing.T) {
	if os.Getenv("HID_TEST_MANUAL_EVENTS") != "1" {
		t.Skip("set HID_TEST_MANUAL_EVENTS=1 to run the unplug/replug test")
	}

	receiver := linuxIntegrationEventReceiver(t)
	defer receiver.Close()

	hint := os.Getenv("HID_TEST_EVENT_HINT")
	t.Log("disconnect the target HID device")
	disconnected := waitForLinuxEvent(t, receiver.Listen(), DeviceEventDisconnected, hint, 60*time.Second)
	t.Log("reconnect the target HID device")
	connected := waitForLinuxEvent(t, receiver.Listen(), DeviceEventConnected, hint, 60*time.Second)

	if disconnected.DeviceInfo.Path == "" {
		t.Fatal("disconnect event has an empty path")
	}
	if connected.DeviceInfo.Path == "" {
		t.Fatal("connect event has an empty path")
	}
	if connected.Err != nil {
		t.Logf("connected metadata is partial: %v", connected.Err)
	}
}
