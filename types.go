package hid

type DeviceInfo struct {
	Path           string // Platform-Specific Device Path
	VendorID       uint16 // Device Vendor ID
	ProductID      uint16 // Device Product ID
	SerialNbr      string // Serial Number
	ReleaseNbr     uint16 // Device Version Number
	MfrStr         string // Manufacturer String
	ProductStr     string // Product String
	UsagePage      uint16 // Usage Page for Device/Interface
	Usage          uint16 // Usage for Device/Interface
	InterfaceNbr   int    // USB Interface Number
	InstanceID     string
	ParentDeviceID string
}

type DeviceInfoFilter func(*DeviceInfo) bool

type EnumerateOption func(*enumerateOptions)

type enumerateOptions struct {
	filters []DeviceInfoFilter
}

func WithDeviceInfoFilter(filter DeviceInfoFilter) EnumerateOption {
	return func(opts *enumerateOptions) {
		if filter != nil {
			opts.filters = append(opts.filters, filter)
		}
	}
}

func newEnumerateOptions(options []EnumerateOption) enumerateOptions {
	var opts enumerateOptions
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	return opts
}

func (opts enumerateOptions) match(info *DeviceInfo) bool {
	for _, filter := range opts.filters {
		if !filter(info) {
			return false
		}
	}
	return true
}
