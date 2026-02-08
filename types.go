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
