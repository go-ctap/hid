package hid

import (
	"context"
	"sync"
)

type ioResult struct {
	n    int
	data []byte
	err  error
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
