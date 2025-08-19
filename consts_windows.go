package hid

const (
	_HIDP_STATUS_SUCCESS                = 0x00110000
	_HIDP_STATUS_INVALID_PREPARSED_DATA = 0xc0110001
	_HIDP_STATUS_USAGE_NOT_FOUND        = 0xc0110004
)

const (
	_DN_ROOT_ENUMERATED = 0x00000001 // Was enumerated by ROOT
	_DN_DRIVER_LOADED   = 0x00000002 // Has Register_Device_Driver
	_DN_ENUM_LOADED     = 0x00000004 // Has Register_Enumerator
	_DN_STARTED         = 0x00000008 // Is currently configured
	_DN_MANUAL          = 0x00000010 // Manually installed
	_DN_NEED_TO_ENUM    = 0x00000020 // May need reenumeration
	_DN_NOT_FIRST_TIME  = 0x00000040 // Has received a config
	_DN_HARDWARE_ENUM   = 0x00000080 // Enum generates hardware ID
	_DN_LIAR            = 0x00000100 // Lied about can reconfig once
	_DN_HAS_MARK        = 0x00000200 // Not CM_Create_DevInst lately
	_DN_HAS_PROBLEM     = 0x00000400 // Need device installer
	_DN_FILTERED        = 0x00000800 // Is filtered
	_DN_MOVED           = 0x00001000 // Has been moved
	_DN_DISABLEABLE     = 0x00002000 // Can be disabled
	_DN_REMOVABLE       = 0x00004000 // Can be removed
	_DN_PRIVATE_PROBLEM = 0x00008000 // Has a private problem
	_DN_MF_PARENT       = 0x00010000 // Multi function parent
	_DN_MF_CHILD        = 0x00020000 // Multi function child
	_DN_WILL_BE_REMOVED = 0x00040000 // DevInst is being removed
)
