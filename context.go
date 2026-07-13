package hid

import (
	"context"
	"io"
)

// ContextReadWriter reads and writes HID reports using a context.
type ContextReadWriter interface {
	Read(context.Context, []byte) (int, error)
	Write(context.Context, []byte) (int, error)
}

// WithContext binds ctx to device and adapts it to io.ReadWriter. Read and
// Write delegate directly to device without adding context checks.
func WithContext(ctx context.Context, device ContextReadWriter) io.ReadWriter {
	return contextReadWriter{ctx: ctx, device: device}
}

type contextReadWriter struct {
	ctx    context.Context
	device ContextReadWriter
}

func (d contextReadWriter) Read(p []byte) (int, error) {
	return d.device.Read(d.ctx, p)
}

func (d contextReadWriter) Write(p []byte) (int, error) {
	return d.device.Write(d.ctx, p)
}
