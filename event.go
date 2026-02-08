package hid

type DeviceEvent struct {
	Type       DeviceEventType
	DeviceInfo *DeviceInfo
	Err        error
}

type DeviceEventType string

const (
	DeviceEventConnected    DeviceEventType = "connected"
	DeviceEventDisconnected DeviceEventType = "disconnected"
)

type EventReceiver interface {
	Listen() <-chan DeviceEvent
	Close() error
}
