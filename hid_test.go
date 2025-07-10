//go:build windows || linux

package hid

import (
	"runtime"
	"testing"

	"github.com/goforj/godump"
	"github.com/stretchr/testify/require"
)

func TestCTAPHID(t *testing.T) {
	var devInfos []*DeviceInfo

	for devInfo, err := range Enumerate() {
		require.NoError(t, err)

		godump.Dump(devInfo)

		if devInfo.UsagePage != 0xf1d0 || devInfo.Usage != 0x01 {
			continue
		}

		devInfos = append(devInfos, devInfo)
	}

	for _, devInfo := range devInfos {
		device, err := OpenPath(devInfo.Path)
		require.NoError(t, err)

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
		require.NoError(t, err)
		if runtime.GOOS == "windows" {
			require.Equal(t, 65, n)
		} else {
			require.Equal(t, 16, n)
		}

		resp := make([]byte, 64)
		n, err = device.Read(resp)
		require.NoError(t, err)
		require.Equal(t, 64, n)

		require.Equal(t, []byte{
			// Broadcast CID
			0xff, 0xff, 0xff, 0xff,
			// CTAPHID_INIT | 0x80 (INIT PACKET BIT)
			0x86,
			// Response size (17 bytes per spec)
			0x00, 0x11,
			// Nonce
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		}, resp[:15])
	}
}
