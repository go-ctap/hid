//go:build linux

package hid

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sys/unix"
)

const linuxPollTimeout = 50

func waitLinuxReadable(ctx context.Context, fd int) error {
	pollFDs := []unix.PollFd{{
		Fd:     int32(fd),
		Events: unix.POLLIN,
	}}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		ready, err := unix.Poll(pollFDs, linuxPollTimeout)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if ready > 0 {
			return nil
		}
	}
}

// runIO serializes operations of one kind. The mutex remains held by the
// operation goroutine when cancellation cannot stop the native call promptly.
func runIO(ctx context.Context, mu *sync.Mutex, operation func() ioResult) ioResult {
	mu.Lock()

	if err := ctx.Err(); err != nil {
		mu.Unlock()

		return ioResult{err: err}
	}

	result := make(chan ioResult, 1)
	go func() {
		defer mu.Unlock()
		result <- operation()
	}()

	select {
	case <-ctx.Done():
		return ioResult{err: ctx.Err()}

	case r := <-result:
		return r
	}
}
