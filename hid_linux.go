package hid

import (
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-ctap/hid/reportparser"
)

func Enumerate(options ...EnumerateOption) iter.Seq2[*DeviceInfo, error] {
	opts := newEnumerateOptions(options)

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
				sysfsDevicePath := filepath.Join("/sys/class/hidraw", name, "device")

				// Parse usage page and usage from report descriptor
				rawDescriptor, err := os.ReadFile(filepath.Join(sysfsDevicePath, "report_descriptor"))
				if err != nil {
					return nil, err
				}
				fillDeviceInfoUsage(info, rawDescriptor)

				// Parse vendor ID, product ID, product name and serial number from uevent
				uevent, err := os.ReadFile(filepath.Join(sysfsDevicePath, "uevent"))
				if err != nil {
					return nil, err
				}
				if err := fillDeviceInfoFromUevent(info, uevent); err != nil {
					return nil, err
				}

				if !opts.match(info) {
					return nil, nil
				}

				return info, nil
			}()
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}

			if info == nil {
				continue
			}

			if !yield(info, nil) {
				return
			}
		}
	}
}

func fillDeviceInfoFromUevent(info *DeviceInfo, uevent []byte) error {
	for _, line := range strings.Split(string(uevent), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		switch key {
		case "HID_ID":
			parts := strings.Split(value, ":")
			if len(parts) != 3 {
				continue
			}
			vendorID, err := strconv.ParseUint(parts[1], 16, 16)
			if err != nil {
				return err
			}
			productID, err := strconv.ParseUint(parts[2], 16, 16)
			if err != nil {
				return err
			}
			info.VendorID = uint16(vendorID)
			info.ProductID = uint16(productID)
		case "HID_NAME":
			info.ProductStr = value
		case "HID_UNIQ":
			info.SerialNbr = value
		}
	}

	return nil
}

func fillDeviceInfoUsage(info *DeviceInfo, rawDescriptor []byte) {
	for _, item := range reportparser.ParseReport(rawDescriptor) {
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
