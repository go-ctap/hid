package hid

type DeviceInfo struct {
	Path       string // Platform-Specific Device Path
	VendorID   uint16 // Device Vendor ID
	ProductID  uint16 // Device Product ID
	MfrStr     string // Manufacturer String
	ProductStr string // Product String
	UsagePage  uint16 // Usage Page for Device/Interface
	Usage      uint16 // Usage for Device/Interface
}
