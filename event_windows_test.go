//go:build windows

package hid

import (
	"os"
	"strings"
	"testing"
	"time"
)

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
