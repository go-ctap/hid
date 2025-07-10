package hid

import (
	"iter"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/go-ctap/hid/reportparser"
)

func ioctlHIDGetDescSize(fd int) (size uint32, err error) {
	_, _, e1 := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.HIDIOCGRDESCSIZE), uintptr(unsafe.Pointer(&size)))
	if e1 != 0 {
		err = e1
	}
	return
}

func Enumerate() iter.Seq2[*DeviceInfo, error] {
	return func(yield func(*DeviceInfo, error) bool) {
		dir, err := os.Open("/sys/class/hidraw")
		if err != nil {
			yield(nil, err)
			return
		}
		defer func() {
			_ = dir.Close()
		}()

		names, err := dir.Readdirnames(0)
		if err != nil {
			yield(nil, err)
			return
		}

		for _, name := range names {
			info, err := func() (*DeviceInfo, error) {
				path := filepath.Join("/dev", filepath.Base(name))
				info := &DeviceInfo{Path: path}

				dev, err := os.Open(info.Path)
				if err != nil {
					return nil, err
				}
				defer func() {
					_ = dev.Close()
				}()

				fd := int(dev.Fd())

				size, err := ioctlHIDGetDescSize(fd)
				if err != nil {
					return nil, err
				}

				rawDescriptor := unix.HIDRawReportDescriptor{
					Size: size,
				}
				if err := unix.IoctlHIDGetDesc(int(dev.Fd()), &rawDescriptor); err != nil {
					return nil, err
				}

				for _, item := range reportparser.ParseReport(rawDescriptor.Value[:rawDescriptor.Size]) {
					switch e := item.(type) {
					case reportparser.UsagePage:
						if info.UsagePage == 0 {
							info.UsagePage = e.Value()
						}
					case reportparser.Usage:
						if info.Usage == 0 {
							info.Usage = e.Value()
						}
					}
				}

				rawInfo, err := unix.IoctlHIDGetRawInfo(int(dev.Fd()))
				if err != nil {
					return nil, err
				}
				info.VendorID = uint16(rawInfo.Vendor)
				info.ProductID = uint16(rawInfo.Product)

				productStr, err := unix.IoctlHIDGetRawName(int(dev.Fd()))
				if err != nil {
					return nil, err
				}
				info.ProductStr = productStr

				return info, nil
			}()
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(info, err) {
				return
			}
		}
	}
}

type Device struct {
	file *os.File
}

func OpenPath(path string) (*Device, error) {
	dev, err := os.OpenFile(path, os.O_RDWR, 0755)
	if err != nil {
		return nil, err
	}

	return &Device{
		file: dev,
	}, nil
}

func (d *Device) Read(b []byte) (int, error) {
	return d.file.Read(b)
}

func (d *Device) Write(b []byte) (int, error) {
	return d.file.Write(b)
}

func (d *Device) Close() error {
	return d.file.Close()
}
