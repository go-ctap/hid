package hid

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDeviceEventQueueBurstIsLosslessAndOrdered(t *testing.T) {
	q := newDeviceEventQueue()
	const eventCount = 4096

	for i := range eventCount {
		event := DeviceEvent{DeviceInfo: &DeviceInfo{Path: fmt.Sprintf("device-%04d", i)}}
		if !q.Send(event) {
			t.Fatalf("Send(%d) rejected before Close", i)
		}
	}

	events := q.Listen()
	for i := range eventCount {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("Listen closed after %d events; want %d", i, eventCount)
			}
			want := fmt.Sprintf("device-%04d", i)
			if event.DeviceInfo == nil || event.DeviceInfo.Path != want {
				t.Fatalf("event %d = %#v; want path %q", i, event, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}

	q.Close()
	if _, ok := <-events; ok {
		t.Fatal("Listen remains open after Close")
	}
}

func TestDeviceEventQueueCloseWithoutReader(t *testing.T) {
	q := newDeviceEventQueue()
	for i := 0; i < 128; i++ {
		if !q.Send(DeviceEvent{}) {
			t.Fatalf("Send(%d) rejected before Close", i)
		}
	}

	closed := make(chan struct{})
	go func() {
		q.Close()
		q.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked without a reader")
	}

	if _, ok := <-q.Listen(); ok {
		t.Fatal("Listen remains open after Close")
	}
	if q.Send(DeviceEvent{}) {
		t.Fatal("Send accepted an event after Close")
	}
}

func TestDeviceEventQueueConcurrentSendAndClose(t *testing.T) {
	q := newDeviceEventQueue()

	const senderCount = 32
	var senders sync.WaitGroup
	var firstSends sync.WaitGroup
	firstSends.Add(senderCount)
	senders.Add(senderCount)
	for i := 0; i < senderCount; i++ {
		go func() {
			defer senders.Done()
			q.Send(DeviceEvent{})
			firstSends.Done()
			for q.Send(DeviceEvent{}) {
			}
		}()
	}

	firstSends.Wait()
	const closerCount = 8
	var closers sync.WaitGroup
	closers.Add(closerCount)
	for i := 0; i < closerCount; i++ {
		go func() {
			defer closers.Done()
			q.Close()
		}()
	}

	finished := make(chan struct{})
	go func() {
		senders.Wait()
		closers.Wait()
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Send and Close did not finish")
	}

	if _, ok := <-q.Listen(); ok {
		t.Fatal("Listen remains open after concurrent Close")
	}
}
