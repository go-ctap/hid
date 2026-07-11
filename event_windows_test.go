//go:build windows

package hid

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReconcileCMStartupEvents(t *testing.T) {
	callbackErr := errors.New("metadata unavailable")
	snapshot := []*DeviceInfo{
		{Path: `\\?\HID#A`, ProductID: 1},
		{Path: `\\?\hid#b`, ProductID: 2},
		{Path: `\\?\hid#stale`, ProductID: 3},
		// SetupAPI can occasionally expose the same interface with different
		// path casing. The startup snapshot must still contain it only once.
		{Path: `\\?\hid#a`, ProductID: 4},
	}
	changes := []DeviceEvent{
		{
			Type:       DeviceEventDisconnected,
			DeviceInfo: &DeviceInfo{Path: `\\?\HID#STALE`},
		},
		{
			Type:       DeviceEventConnected,
			DeviceInfo: &DeviceInfo{Path: `\\?\hid#new`, ProductID: 5},
			Err:        callbackErr,
		},
		{
			Type:       DeviceEventDisconnected,
			DeviceInfo: &DeviceInfo{Path: `\\?\HID#NEW`},
		},
		{
			Type:       DeviceEventDisconnected,
			DeviceInfo: &DeviceInfo{Path: `\\?\HID#B`},
		},
		{
			Type:       DeviceEventConnected,
			DeviceInfo: &DeviceInfo{Path: `\\?\hid#b`, ProductID: 6},
			Err:        callbackErr,
		},
	}

	events := reconcileCMStartupEvents(snapshot, changes)
	if len(events) != 2 {
		t.Fatalf("got %d startup events, want 2: %#v", len(events), events)
	}

	if got := events[0]; got.Type != DeviceEventConnected || got.DeviceInfo == nil ||
		!strings.EqualFold(got.DeviceInfo.Path, `\\?\hid#a`) || got.DeviceInfo.ProductID != 4 {
		t.Fatalf("first event = %#v, want the de-duplicated A snapshot entry", got)
	}
	if got := events[1]; got.Type != DeviceEventConnected || got.DeviceInfo == nil ||
		!strings.EqualFold(got.DeviceInfo.Path, `\\?\hid#b`) || got.DeviceInfo.ProductID != 6 ||
		!errors.Is(got.Err, callbackErr) {
		t.Fatalf("second event = %#v, want the latest B callback", got)
	}

	for _, event := range events {
		if event.DeviceInfo != nil && strings.EqualFold(event.DeviceInfo.Path, `\\?\hid#stale`) {
			t.Fatalf("stale enumeration entry was resurrected: %#v", event)
		}
		if event.DeviceInfo != nil && strings.EqualFold(event.DeviceInfo.Path, `\\?\hid#new`) {
			t.Fatalf("removed startup arrival was retained: %#v", event)
		}
	}
}

func TestCMRemovalUsesCachedDeviceInfo(t *testing.T) {
	queue := newDeviceEventQueue()
	defer queue.Close()

	cached := &DeviceInfo{
		Path:       `\\?\hid#token`,
		VendorID:   0x1050,
		ProductID:  0x0407,
		UsagePage:  0xf1d0,
		Usage:      1,
		ProductStr: "Security Key",
	}
	receiver := &cmEventReceiver{
		events: queue,
		devices: map[string]*DeviceInfo{
			cmDevicePathKey(cached.Path): cached,
		},
	}

	receiver.onPathAction(_CM_NOTIFY_ACTION_DEVICEINTERFACEREMOVAL, `\\?\HID#TOKEN`)
	select {
	case event := <-queue.Listen():
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
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cached removal event")
	}

	// A repeated/unknown removal is still a native callback and must not be
	// dropped; without cached metadata its callback path is the fallback.
	receiver.onPathAction(_CM_NOTIFY_ACTION_DEVICEINTERFACEREMOVAL, `\\?\HID#TOKEN`)
	select {
	case event := <-queue.Listen():
		if event.DeviceInfo == nil || event.DeviceInfo.Path != `\\?\HID#TOKEN` ||
			event.DeviceInfo.UsagePage != 0 || event.DeviceInfo.Usage != 0 {
			t.Fatalf("fallback event info = %#v", event.DeviceInfo)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fallback removal event")
	}
}

func assertEventReceiverLifecycle(t *testing.T, newReceiver func() (EventReceiver, error)) {
	t.Helper()

	er, err := newReceiver()
	if err != nil {
		t.Fatal(err)
	}

	ch := er.Listen()

	if err := er.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := er.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("listen channel was not closed after Close()")
		}
	}
}

func TestEventLifecycle(t *testing.T) {
	assertEventReceiverLifecycle(t, Events)
}

func waitForEvent(t *testing.T, ch <-chan DeviceEvent, wantType DeviceEventType, pathHint string, timeout time.Duration) DeviceEvent {
	t.Helper()

	deadline := time.After(timeout)
	normalizedHint := strings.ToLower(pathHint)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("event channel closed before expected event arrived")
			}

			path := ""
			if ev.DeviceInfo != nil {
				path = ev.DeviceInfo.Path
			}

			matched := ev.Type == wantType
			if matched && normalizedHint != "" {
				matched = strings.Contains(strings.ToLower(path), normalizedHint)
			}
			if !matched {
				t.Logf("skip event type=%s path=%q err=%v", ev.Type, path, ev.Err)
				continue
			}

			t.Logf("got event type=%s path=%q err=%v", ev.Type, path, ev.Err)
			return ev
		case <-deadline:
			t.Fatalf("timeout waiting for %s event (hint=%q)", wantType, pathHint)
		}
	}
}

func TestEventManualDisconnectConnect(t *testing.T) {
	if os.Getenv("HID_TEST_MANUAL_EVENTS") != "1" {
		t.Skip("manual test; set HID_TEST_MANUAL_EVENTS=1 to enable")
	}

	pathHint := os.Getenv("HID_TEST_EVENT_HINT")
	er, err := Events()
	if err != nil {
		if strings.Contains(err.Error(), "CM_Register_Notification failed") {
			t.Skipf("events backend unavailable in this environment: %v", err)
		}
		t.Fatal(err)
	}
	defer er.Close()

	t.Logf("manual test started (hint=%q)", pathHint)

	t.Log("step 1: disconnect target USB HID device now")
	disc := waitForEvent(t, er.Listen(), DeviceEventDisconnected, pathHint, 60*time.Second)

	t.Log("step 2: reconnect the same USB HID device now")
	conn := waitForEvent(t, er.Listen(), DeviceEventConnected, pathHint, 60*time.Second)

	if disc.DeviceInfo == nil || disc.DeviceInfo.Path == "" {
		t.Fatal("disconnect event does not include device path")
	}
	if conn.DeviceInfo == nil || conn.DeviceInfo.Path == "" {
		t.Fatal("connect event does not include device path")
	}
	if pathHint != "" && !strings.Contains(strings.ToLower(conn.DeviceInfo.Path), strings.ToLower(pathHint)) {
		t.Fatalf("connect event path %q does not match hint %q", conn.DeviceInfo.Path, pathHint)
	}

	// Metadata retrieval can fail due to ACL/driver policy; the event itself is still valid.
	if conn.Err != nil {
		t.Logf("connect metadata is partial: %v", conn.Err)
	}
}
