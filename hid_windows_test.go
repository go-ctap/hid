package hid

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestGetHidGuid(t *testing.T) {
	guid, err := getHidGuid()
	if err != nil {
		t.Fatal(err)
	}

	if guid == nil {
		t.Fatal("getHidGuid returned nil")
	}
	if *guid == (windows.GUID{}) {
		t.Fatal("getHidGuid returned an empty GUID")
	}
}

func TestReadCancellationCallsCancelIoEx(t *testing.T) {
	originalReadFile := windowsReadFile
	originalCancelIoEx := windowsCancelIoEx
	t.Cleanup(func() {
		windowsReadFile = originalReadFile
		windowsCancelIoEx = originalCancelIoEx
	})

	started := make(chan struct{})
	release := make(chan struct{})
	cancelCalled := make(chan *windows.Overlapped, 1)
	windowsReadFile = func(_ windows.Handle, _ []byte, _ *uint32, _ *windows.Overlapped) error {
		close(started)
		<-release
		return windows.ERROR_OPERATION_ABORTED
	}
	windowsCancelIoEx = func(_ windows.Handle, overlapped *windows.Overlapped) error {
		cancelCalled <- overlapped
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	device := &Device{hFile: 1, inputReportByteLength: 64, readTimeout: windows.INFINITE}
	done := make(chan error, 1)
	go func() {
		_, err := device.Read(ctx, make([]byte, 64))
		done <- err
	}()

	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not return after cancellation")
	}

	select {
	case overlapped := <-cancelCalled:
		if overlapped == nil {
			t.Fatal("CancelIoEx was called without the operation's OVERLAPPED")
		}
	case <-time.After(time.Second):
		t.Fatal("CancelIoEx was not called")
	}
	close(release)
}
