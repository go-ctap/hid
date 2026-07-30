package hid

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIOReturnPermissionError(t *testing.T) {
	err := ioReturnError("IOHIDDeviceOpen", kIOReturnNotPermitted)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("errors.Is(%v, os.ErrPermission) = false", err)
	}
}

func TestEventLifecycle(t *testing.T) {
	receiver, err := Watch()
	if err != nil {
		t.Fatal(err)
	}
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

func TestWatchCapturesInitialSnapshot(t *testing.T) {
	receiver, err := Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	expected := make(map[string]struct{})
	for info, err := range Enumerate() {
		if err != nil {
			t.Fatal(err)
		}
		expected[info.Path] = struct{}{}
	}
	seen := make(map[string]struct{}, len(expected))
	for _, device := range receiver.Snapshot().Devices {
		if device.DeviceInfo == nil {
			t.Fatal("initial snapshot contains nil DeviceInfo")
		}
		path := device.DeviceInfo.Path
		if _, duplicate := seen[path]; duplicate {
			t.Fatalf("duplicate initial device for %q", path)
		}
		seen[path] = struct{}{}
		delete(expected, path)
	}
	if len(expected) != 0 {
		t.Fatalf("initial snapshot is missing %d device(s): %v", len(expected), expected)
	}
}

func TestEventRemovalUsesCachedDeviceInfo(t *testing.T) {
	queue := newDeviceEventQueue()
	receiver := &darwinEventReceiver{
		events: queue,
		devices: map[ioHIDDeviceRef]*DeviceInfo{
			123: {
				Path:       "cached-path",
				VendorID:   0x1050,
				ProductID:  0x0407,
				UsagePage:  0xf1d0,
				Usage:      1,
				ProductStr: "cached product",
			},
		},
	}
	defer queue.Close()

	receiver.deviceDisconnected(kIOReturnSuccess, 123)
	select {
	case event := <-queue.Listen():
		if event.Type != DeviceEventDisconnected {
			t.Fatalf("event type = %q", event.Type)
		}
		if event.MetadataErr != nil {
			t.Fatalf("event error = %v", event.MetadataErr)
		}
		if event.DeviceInfo == nil || event.DeviceInfo.Path != "cached-path" || event.DeviceInfo.UsagePage != 0xf1d0 {
			t.Fatalf("event info = %#v", event.DeviceInfo)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for removal event")
	}
}

func waitForDarwinEvent(t *testing.T, events <-chan DeviceEvent, eventType DeviceEventType, hint string, timeout time.Duration) DeviceEvent {
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
			t.Logf("event type=%s path=%q product=%q err=%v", event.Type, info.Path, info.ProductStr, event.MetadataErr)
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

	receiver, err := Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	hint := os.Getenv("HID_TEST_EVENT_HINT")
	t.Log("disconnect the target HID device")
	disconnected := waitForDarwinEvent(t, receiver.Listen(), DeviceEventDisconnected, hint, 60*time.Second)
	t.Log("reconnect the target HID device")
	connected := waitForDarwinEvent(t, receiver.Listen(), DeviceEventConnected, hint, 60*time.Second)

	if disconnected.DeviceInfo.Path == "" {
		t.Fatal("disconnect event has an empty path")
	}
	if connected.DeviceInfo.Path == "" {
		t.Fatal("connect event has an empty path")
	}
	if connected.MetadataErr != nil {
		t.Logf("connected metadata is partial: %v", connected.MetadataErr)
	}
}
