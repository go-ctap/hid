# go-hid

Working prototype to prove that you don't need `cgo` to work with HID devices.
Currently, Windows, macOS and Linux are supported.

Developed as part of [go-ctaphid](https://github.com/go-ctap/ctaphid).

## Status

- [x] Windows
  - [x] Enumerate
  - [x] Connect/disconnect events
  - [x] Open
    - [x] Read
    - [x] Write
- [x] macOS
  - [x] Enumerate
  - [x] Connect/disconnect events
  - [x] Open
      - [x] Read
      - [x] Write
- [x] Linux
  - [x] Enumerate
  - [x] Connect/disconnect events
  - [x] Open
     - [x] Read
     - [x] Write

## HID connection events

`Events` is available on Windows, macOS and Linux. A new receiver first publishes a
`connected` event for every HID device that is already present, then continues
with live `connected` and `disconnected` events. Events are ordered and queued
until they are consumed or the receiver is closed.

```go
receiver, err := hid.Events()
if err != nil {
	log.Fatal(err)
}
defer receiver.Close()

fidoDevices := make(map[string]*hid.DeviceInfo)
for event := range receiver.Listen() {
	if event.DeviceInfo == nil {
		continue
	}

	switch event.Type {
	case hid.DeviceEventConnected:
		if event.DeviceInfo.UsagePage == 0xf1d0 && event.DeviceInfo.Usage == 1 {
			fidoDevices[event.DeviceInfo.Path] = event.DeviceInfo
		}
	case hid.DeviceEventDisconnected:
		delete(fidoDevices, event.DeviceInfo.Path)
	}

	// A non-nil Err means the state change happened, but some metadata may be partial.
	if event.Err != nil {
		log.Printf("HID event metadata: %v", event.Err)
	}
}
```

On macOS the event receiver only enumerates devices and reads their properties;
it does not open them and does not require administrator privileges. Opening a
protected HID device for I/O is a separate operation and can still be denied by
macOS policy or an application sandbox.

On Linux the receiver listens to kernel `hidraw` uevents and does not open the
device. A `connected` event can arrive before udev has created the corresponding
`/dev/hidrawN` node or applied its final access permissions.
