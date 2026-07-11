package hid

import "sync"

type DeviceEvent struct {
	Type       DeviceEventType
	DeviceInfo *DeviceInfo
	Err        error
}

type DeviceEventType string

const (
	DeviceEventConnected    DeviceEventType = "connected"
	DeviceEventDisconnected DeviceEventType = "disconnected"
)

type EventReceiver interface {
	Listen() <-chan DeviceEvent
	Close() error
}

// deviceEventQueue decouples native device callbacks from event consumers.
// Send only retains the event and wakes the forwarding goroutine; it never
// waits for a consumer to receive from Listen.
type deviceEventQueue struct {
	mu      sync.Mutex
	pending []DeviceEvent
	closed  bool

	out     chan DeviceEvent
	wake    chan struct{}
	done    chan struct{}
	stopped chan struct{}

	closeOnce sync.Once
}

func newDeviceEventQueue() *deviceEventQueue {
	q := &deviceEventQueue{
		out:     make(chan DeviceEvent),
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go q.run()
	return q
}

// Send enqueues an event without waiting for a listener. It reports false once
// closing has started; calls racing with Close are safe.
func (q *deviceEventQueue) Send(event DeviceEvent) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.pending = append(q.pending, event)
	q.mu.Unlock()

	select {
	case q.wake <- struct{}{}:
	default:
	}

	return true
}

func (q *deviceEventQueue) Listen() <-chan DeviceEvent {
	return q.out
}

// Close stops accepting events and cancels any delivery blocked on an absent
// consumer. It waits for out to be closed, but never waits for out to drain.
func (q *deviceEventQueue) Close() {
	q.closeOnce.Do(func() {
		q.mu.Lock()
		q.closed = true
		q.pending = nil
		q.mu.Unlock()

		close(q.done)
		<-q.stopped
	})
}

func (q *deviceEventQueue) run() {
	defer close(q.stopped)
	defer close(q.out)

	for {
		q.mu.Lock()
		if len(q.pending) == 0 {
			q.mu.Unlock()
			select {
			case <-q.wake:
				continue
			case <-q.done:
				return
			}
		}

		event := q.pending[0]
		q.pending[0] = DeviceEvent{}
		if len(q.pending) == 1 {
			q.pending = nil
		} else {
			q.pending = q.pending[1:]
		}
		q.mu.Unlock()

		select {
		case q.out <- event:
		case <-q.done:
			return
		}
	}
}
