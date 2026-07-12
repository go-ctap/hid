//go:build windows || linux || darwin

package hid

import (
	"errors"
	"os"
	"runtime"
	"slices"
	"testing"
)

func TestCTAPHID(t *testing.T) {
	if os.Getenv("HID_TEST_CTAPHID") != "1" {
		t.Skip("set HID_TEST_CTAPHID=1 to run the hardware test")
	}

	var devInfos []*DeviceInfo

	for devInfo, err := range Enumerate(
		WithUsagePage(0xf1d0),
		WithUsage(0x01),
	) {
		if err != nil {
			t.Error(err)
			continue
		}

		t.Logf("device info: %+v", devInfo)

		devInfos = append(devInfos, devInfo)
	}

	for _, devInfo := range devInfos {
		device, err := OpenPath(devInfo.Path)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := device.Close(); err != nil {
				t.Fatal(err)
			}
		}()

		n, err := device.Write([]byte{
			// ReportID
			0x00,
			// Broadcast CID
			0xff, 0xff, 0xff, 0xff,
			// CTAPHID_INIT | 0x80 (INIT PACKET BIT)
			0x86,
			// Nonce size
			0x00, 0x08,
			// Nonce
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		})
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS == "windows" {
			if n != 65 {
				t.Fatalf("Write() = %d bytes, want 65", n)
			}
		} else {
			if n != 16 {
				t.Fatalf("Write() = %d bytes, want 16", n)
			}
		}

		resp := make([]byte, 64)
		n, err = device.Read(resp)
		if err != nil {
			t.Fatal(err)
		}
		if n != 64 {
			t.Fatalf("Read() = %d bytes, want 64", n)
		}

		want := []byte{
			// Broadcast CID
			0xff, 0xff, 0xff, 0xff,
			// CTAPHID_INIT | 0x80 (INIT PACKET BIT)
			0x86,
			// Response size (17 bytes per spec)
			0x00, 0x11,
			// Nonce
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		}
		if !slices.Equal(resp[:15], want) {
			t.Fatalf("response prefix = %x, want %x", resp[:15], want)
		}
	}
}
