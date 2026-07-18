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

type ioResult struct {
	n    int
	data []byte
	err  error
}

type EnumerateOption func(*enumerateOptions)

type enumerateOptions struct {
	path           *string
	vendorID       *uint16
	productID      *uint16
	serialNbr      *string
	releaseNbr     *uint16
	mfrStr         *string
	productStr     *string
	usagePage      *uint16
	usage          *uint16
	interfaceNbr   *int
	instanceID     *string
	parentDeviceID *string
}

func WithPath(path string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.path = &path
	}
}

func WithVendorID(vendorID uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.vendorID = &vendorID
	}
}

func WithProductID(productID uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.productID = &productID
	}
}

func WithSerialNumber(serialNbr string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.serialNbr = &serialNbr
	}
}

func WithReleaseNumber(releaseNbr uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.releaseNbr = &releaseNbr
	}
}

func WithManufacturerString(mfrStr string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.mfrStr = &mfrStr
	}
}

func WithProductString(productStr string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.productStr = &productStr
	}
}

func WithUsagePage(usagePage uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.usagePage = &usagePage
	}
}

func WithUsage(usage uint16) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.usage = &usage
	}
}

func WithInterfaceNumber(interfaceNbr int) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.interfaceNbr = &interfaceNbr
	}
}

func WithInstanceID(instanceID string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.instanceID = &instanceID
	}
}

func WithParentDeviceID(parentDeviceID string) EnumerateOption {
	return func(opts *enumerateOptions) {
		opts.parentDeviceID = &parentDeviceID
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
	if opts.path != nil && info.Path != *opts.path {
		return false
	}
	if opts.vendorID != nil && info.VendorID != *opts.vendorID {
		return false
	}
	if opts.productID != nil && info.ProductID != *opts.productID {
		return false
	}
	if opts.serialNbr != nil && info.SerialNbr != *opts.serialNbr {
		return false
	}
	if opts.releaseNbr != nil && info.ReleaseNbr != *opts.releaseNbr {
		return false
	}
	if opts.mfrStr != nil && info.MfrStr != *opts.mfrStr {
		return false
	}
	if opts.productStr != nil && info.ProductStr != *opts.productStr {
		return false
	}
	if opts.usagePage != nil && info.UsagePage != *opts.usagePage {
		return false
	}
	if opts.usage != nil && info.Usage != *opts.usage {
		return false
	}
	if opts.interfaceNbr != nil && info.InterfaceNbr != *opts.interfaceNbr {
		return false
	}
	if opts.instanceID != nil && info.InstanceID != *opts.instanceID {
		return false
	}
	if opts.parentDeviceID != nil && info.ParentDeviceID != *opts.parentDeviceID {
		return false
	}
	return true
}
