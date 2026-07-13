package hid

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunIOSuccess(t *testing.T) {
	var mu sync.Mutex
	result := runIO(context.Background(), &mu, func() ioResult {
		return ioResult{n: 3}
	})
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.n != 3 {
		t.Fatalf("result = %d bytes, want 3", result.n)
	}
}

func TestRunIOCancellationReturnsPromptly(t *testing.T) {
	var mu sync.Mutex
	started := make(chan struct{})
	release := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan ioResult, 1)
	go func() {
		done <- runIO(ctx, &mu, func() ioResult {
			close(started)
			<-release
			return ioResult{}
		})
	}()

	<-started
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("operation did not return after cancellation")
	}

	close(release)
}

func TestRunIODeadlineExceeded(t *testing.T) {
	var mu sync.Mutex
	release := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	result := runIO(ctx, &mu, func() ioResult {
		<-release
		return ioResult{}
	})
	if !errors.Is(result.err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", result.err)
	}
	close(release)
}

func TestRunIODoesNotStartAfterCancellationWhileQueued(t *testing.T) {
	var mu sync.Mutex
	var calls atomic.Int32
	mu.Lock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan ioResult, 1)
	go func() {
		done <- runIO(ctx, &mu, func() ioResult {
			calls.Add(1)
			return ioResult{}
		})
	}()

	cancel()
	mu.Unlock()
	result := <-done
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", result.err)
	}
	if calls.Load() != 0 {
		t.Fatal("operation started after cancellation while queued")
	}
}
