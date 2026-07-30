//go:build linux

package hid

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDeviceReadCancellationLeavesReadUsable(t *testing.T) {
	var pipe [2]int
	if err := unix.Pipe2(pipe[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		t.Fatal(err)
	}
	device := &Device{file: os.NewFile(uintptr(pipe[0]), "test pipe")}
	defer func() {
		_ = device.Close()
		_ = unix.Close(pipe[1])
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := device.Read(ctx, make([]byte, 1))
		done <- err
	}()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("poll did not stop after cancellation")
	}

	if _, err := unix.Write(pipe[1], []byte{1}); err != nil {
		t.Fatal(err)
	}
	readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
	defer cancelRead()
	buffer := make([]byte, 1)
	n, err := device.Read(readCtx, buffer)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || buffer[0] != 1 {
		t.Fatalf("Read() = %d bytes %v, want [1]", n, buffer)
	}
}

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
