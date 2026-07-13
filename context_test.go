package hid

import (
	"context"
	"testing"
)

type contextReadWriterStub struct {
	readContext  context.Context
	writeContext context.Context
	readBuffer   []byte
	writeBuffer  []byte
}

func (d *contextReadWriterStub) Read(ctx context.Context, p []byte) (int, error) {
	d.readContext = ctx
	d.readBuffer = p
	return len(p), nil
}

func (d *contextReadWriterStub) Write(ctx context.Context, p []byte) (int, error) {
	d.writeContext = ctx
	d.writeBuffer = p
	return len(p), nil
}

func TestWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	device := &contextReadWriterStub{}
	bound := WithContext(ctx, device)

	readBuffer := make([]byte, 64)
	read, err := bound.Read(readBuffer)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if read != len(readBuffer) {
		t.Fatalf("Read returned %d bytes, want %d", read, len(readBuffer))
	}
	if device.readContext != ctx {
		t.Fatal("Read did not receive the bound context")
	}
	if len(device.readBuffer) != len(readBuffer) || &device.readBuffer[0] != &readBuffer[0] {
		t.Fatal("Read did not receive the original buffer")
	}

	writeBuffer := make([]byte, 64)
	written, err := bound.Write(writeBuffer)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if written != len(writeBuffer) {
		t.Fatalf("Write returned %d bytes, want %d", written, len(writeBuffer))
	}
	if device.writeContext != ctx {
		t.Fatal("Write did not receive the bound context")
	}
	if len(device.writeBuffer) != len(writeBuffer) || &device.writeBuffer[0] != &writeBuffer[0] {
		t.Fatal("Write did not receive the original buffer")
	}
}
