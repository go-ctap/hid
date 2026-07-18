//go:build windows || linux || darwin

package hid

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"slices"
	"strconv"
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

		n, err := device.Write(context.Background(), []byte{
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
		n, err = device.Read(context.Background(), resp)
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

		pings := 0
		if value := os.Getenv("HID_TEST_CTAPHID_PINGS"); value != "" {
			pings, err = strconv.Atoi(value)
			if err != nil || pings < 0 {
				t.Fatalf("invalid HID_TEST_CTAPHID_PINGS value %q", value)
			}
		}

		var channel [4]byte
		copy(channel[:], resp[15:19])
		for i := 0; i < pings; i++ {
			payload := make([]byte, 8)
			binary.BigEndian.PutUint64(payload, uint64(i))
			request := []byte{
				0,
				channel[0], channel[1], channel[2], channel[3],
				0x81,
				0, byte(len(payload)),
			}
			request = append(request, payload...)

			n, err = device.Write(context.Background(), request)
			if err != nil {
				t.Fatalf("ping %d write: %v", i, err)
			}
			if runtime.GOOS == "windows" {
				if n != 65 {
					t.Fatalf("ping %d Write() = %d bytes, want 65", i, n)
				}
			} else if n != len(request) {
				t.Fatalf("ping %d Write() = %d bytes, want %d", i, n, len(request))
			}

			n, err = device.Read(context.Background(), resp)
			if err != nil {
				t.Fatalf("ping %d read: %v", i, err)
			}
			if n != 64 {
				t.Fatalf("ping %d Read() = %d bytes, want 64", i, n)
			}
			if !slices.Equal(resp[:7], request[1:8]) || !slices.Equal(resp[7:15], payload) {
				t.Fatalf("ping %d response = %x", i, resp[:15])
			}
		}
	}
}
