package hid

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	linuxUeventGroup      = 1
	linuxUeventBufferSize = 64 * 1024
	linuxUeventRcvbufSize = 1024 * 1024
	linuxUeventBatchSize  = 64
)

type linuxUevent struct {
	eventType DeviceEventType
	device    string
}

type linuxEventReceiver struct {
	events   *deviceEventQueue
	socketFD int
	wakeFD   int
	stopped  chan struct{}

	loadDeviceInfo func(string) (*DeviceInfo, error)

	mu            sync.Mutex
	closed        bool
	initializing  bool
	startupEvents []DeviceEvent
	devices       map[string]*DeviceInfo
	runErr        error
	closeErr      error

	closeOnce sync.Once
}

func (er *linuxEventReceiver) Listen() <-chan DeviceEvent {
	return er.events.Listen()
}

func (er *linuxEventReceiver) Close() error {
	er.closeOnce.Do(func() {
		er.mu.Lock()
		wasClosed := er.closed
		er.closed = true
		if !wasClosed {
			if err := signalLinuxEventFD(er.wakeFD); err != nil && !errors.Is(err, unix.EBADF) {
				er.closeErr = errors.Join(er.closeErr, fmt.Errorf("wake Linux HID event receiver: %w", err))
			}
		}
		er.mu.Unlock()

		<-er.stopped
	})

	er.mu.Lock()
	defer er.mu.Unlock()
	return errors.Join(er.runErr, er.closeErr)
}

func signalLinuxEventFD(fd int) error {
	var wake [8]byte
	binary.NativeEndian.PutUint64(wake[:], 1)
	for {
		n, err := unix.Write(fd, wake[:])
		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case errors.Is(err, unix.EAGAIN):
			// A saturated eventfd is already readable, so the receiver is awake.
			return nil
		case err != nil:
			return err
		case n != len(wake):
			return fmt.Errorf("short eventfd write: wrote %d bytes", n)
		default:
			return nil
		}
	}
}

func (er *linuxEventReceiver) run() {
	runErr := er.readUevents()

	er.mu.Lock()
	if runErr != nil {
		er.runErr = errors.Join(er.runErr, runErr)
	}
	er.closed = true

	var closeErr error
	if err := unix.Close(er.socketFD); err != nil && !errors.Is(err, unix.EBADF) {
		closeErr = errors.Join(closeErr, fmt.Errorf("close Linux uevent socket: %w", err))
	}
	if err := unix.Close(er.wakeFD); err != nil && !errors.Is(err, unix.EBADF) {
		closeErr = errors.Join(closeErr, fmt.Errorf("close Linux HID event wake descriptor: %w", err))
	}
	er.closeErr = errors.Join(er.closeErr, closeErr)
	er.mu.Unlock()

	er.events.Close()
	close(er.stopped)
}

func (er *linuxEventReceiver) readUevents() error {
	pollFDs := []unix.PollFd{
		{Fd: int32(er.socketFD), Events: unix.POLLIN},
		{Fd: int32(er.wakeFD), Events: unix.POLLIN},
	}
	message := make([]byte, linuxUeventBufferSize)

	for {
		if er.isClosed() {
			return nil
		}

		pollFDs[0].Revents = 0
		pollFDs[1].Revents = 0
		if _, err := unix.Poll(pollFDs, -1); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return fmt.Errorf("poll Linux HID uevents: %w", err)
		}

		if pollFDs[1].Revents != 0 {
			return nil
		}

		socketEvents := pollFDs[0].Revents
		if socketEvents&(unix.POLLIN|unix.POLLERR) != 0 {
			if err := er.drainUevents(message); err != nil {
				return err
			}
		}
		if socketEvents&(unix.POLLHUP|unix.POLLNVAL) != 0 {
			return fmt.Errorf("Linux HID uevent socket poll failure: revents=0x%x", uint16(socketEvents))
		}
	}
}

func (er *linuxEventReceiver) drainUevents(message []byte) error {
	for range linuxUeventBatchSize {
		if er.isClosed() {
			return nil
		}

		n, _, flags, sender, err := unix.Recvmsg(er.socketFD, message, nil, unix.MSG_DONTWAIT)
		if err != nil {
			switch {
			case errors.Is(err, unix.EINTR):
				continue
			case errors.Is(err, unix.EAGAIN), errors.Is(err, unix.EWOULDBLOCK):
				return nil
			case errors.Is(err, unix.ENOBUFS):
				// Continuing would expose an inconsistent device cache.
				return fmt.Errorf("receive Linux HID uevent: kernel dropped messages: %w", err)
			default:
				return fmt.Errorf("receive Linux HID uevent: %w", err)
			}
		}
		if flags&unix.MSG_TRUNC != 0 {
			return errors.New("receive Linux HID uevent: message was truncated")
		}
		if n == 0 || !isKernelUeventSender(sender) {
			continue
		}

		uevent, ok := parseLinuxUevent(message[:n])
		if ok {
			er.onUevent(uevent)
		}
	}

	return nil
}

func (er *linuxEventReceiver) isClosed() bool {
	er.mu.Lock()
	defer er.mu.Unlock()
	return er.closed
}

func isKernelUeventSender(sender unix.Sockaddr) bool {
	netlink, ok := sender.(*unix.SockaddrNetlink)
	return ok && netlink.Pid == 0
}

func parseLinuxUevent(message []byte) (linuxUevent, bool) {
	var action string
	var headerAction string
	var subsystem string
	var device string
	var devicePath string

	for index, field := range strings.Split(string(message), "\x00") {
		if field == "" {
			continue
		}

		key, value, ok := strings.Cut(field, "=")
		if !ok {
			if index == 0 {
				headerAction, _, _ = strings.Cut(field, "@")
			}
			continue
		}

		switch key {
		case "ACTION":
			action = value
		case "SUBSYSTEM":
			subsystem = value
		case "DEVNAME":
			device = value
		case "DEVPATH":
			devicePath = value
		}
	}

	if action == "" {
		action = headerAction
	}
	if subsystem != "hidraw" {
		return linuxUevent{}, false
	}
	if device == "" && devicePath != "" {
		if slash := strings.LastIndexByte(devicePath, '/'); slash >= 0 {
			device = devicePath[slash+1:]
		} else {
			device = devicePath
		}
	}
	if !isLinuxHIDRawName(device) {
		return linuxUevent{}, false
	}

	switch action {
	case "add":
		return linuxUevent{eventType: DeviceEventConnected, device: device}, true
	case "remove":
		return linuxUevent{eventType: DeviceEventDisconnected, device: device}, true
	default:
		return linuxUevent{}, false
	}
}

func isLinuxHIDRawName(name string) bool {
	const prefix = "hidraw"
	if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
		return false
	}
	for _, char := range name[len(prefix):] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func linuxHIDRawPath(name string) string {
	return filepath.Join(linuxDeviceDir, name)
}

func cloneLinuxDeviceInfo(info *DeviceInfo) *DeviceInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	return &cloned
}

func (er *linuxEventReceiver) onUevent(uevent linuxUevent) {
	if !isLinuxHIDRawName(uevent.device) ||
		(uevent.eventType != DeviceEventConnected && uevent.eventType != DeviceEventDisconnected) {
		return
	}
	path := linuxHIDRawPath(uevent.device)

	er.mu.Lock()
	if er.closed {
		er.mu.Unlock()
		return
	}
	if uevent.eventType == DeviceEventDisconnected {
		info := cloneLinuxDeviceInfo(er.devices[path])
		if info == nil {
			info = &DeviceInfo{Path: path}
		}
		event := DeviceEvent{
			Type:       DeviceEventDisconnected,
			DeviceInfo: info,
		}
		er.publishOrStageLocked(event)
		er.mu.Unlock()
		return
	}
	if !er.initializing {
		if _, exists := er.devices[path]; exists {
			er.mu.Unlock()
			return
		}
	}
	loadDeviceInfo := er.loadDeviceInfo
	er.mu.Unlock()

	info := &DeviceInfo{Path: path}
	var eventErr error
	if loadDeviceInfo != nil {
		loaded, err := loadDeviceInfo(uevent.device)
		if loaded != nil {
			info = cloneLinuxDeviceInfo(loaded)
		}
		if err != nil {
			eventErr = fmt.Errorf("read HID metadata for %s: %w", path, err)
		}
	}
	info.Path = path

	er.mu.Lock()
	defer er.mu.Unlock()
	if er.closed {
		return
	}
	if !er.initializing {
		if _, exists := er.devices[path]; exists {
			return
		}
	}
	er.publishOrStageLocked(DeviceEvent{
		Type:       DeviceEventConnected,
		DeviceInfo: info,
		Err:        eventErr,
	})
}

func (er *linuxEventReceiver) publishOrStageLocked(event DeviceEvent) {
	if er.initializing {
		er.startupEvents = append(er.startupEvents, event)
		return
	}

	path := event.DeviceInfo.Path
	switch event.Type {
	case DeviceEventConnected:
		er.devices[path] = cloneLinuxDeviceInfo(event.DeviceInfo)
	case DeviceEventDisconnected:
		delete(er.devices, path)
	}
	er.events.Send(event)
}

func reconcileLinuxStartupEvents(snapshot, changes []DeviceEvent) []DeviceEvent {
	state := make(map[string]DeviceEvent, len(snapshot)+len(changes))
	order := make([]string, 0, len(snapshot)+len(changes))
	ordered := make(map[string]struct{}, len(snapshot)+len(changes))

	put := func(event DeviceEvent) {
		if event.Type != DeviceEventConnected || event.DeviceInfo == nil || event.DeviceInfo.Path == "" {
			return
		}
		path := event.DeviceInfo.Path
		if _, ok := ordered[path]; !ok {
			ordered[path] = struct{}{}
			order = append(order, path)
		}
		state[path] = event
	}

	for _, event := range snapshot {
		put(event)
	}
	for _, event := range changes {
		if event.DeviceInfo == nil || event.DeviceInfo.Path == "" {
			continue
		}
		switch event.Type {
		case DeviceEventConnected:
			put(event)
		case DeviceEventDisconnected:
			delete(state, event.DeviceInfo.Path)
		}
	}

	events := make([]DeviceEvent, 0, len(state))
	for _, path := range order {
		if event, ok := state[path]; ok {
			events = append(events, event)
		}
	}
	return events
}

func (er *linuxEventReceiver) publishStartup(snapshot []DeviceEvent) {
	er.mu.Lock()
	defer er.mu.Unlock()
	if er.closed {
		return
	}

	for _, event := range reconcileLinuxStartupEvents(snapshot, er.startupEvents) {
		event.DeviceInfo = cloneLinuxDeviceInfo(event.DeviceInfo)
		er.devices[event.DeviceInfo.Path] = cloneLinuxDeviceInfo(event.DeviceInfo)
		er.events.Send(event)
	}
	er.startupEvents = nil
	er.initializing = false
}

func linuxInitialEventSnapshot() ([]DeviceEvent, error) {
	names, err := linuxHIDRawNames()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	events := make([]DeviceEvent, 0, len(names))
	for _, name := range names {
		if !isLinuxHIDRawName(name) {
			continue
		}

		path := linuxHIDRawPath(name)
		info, infoErr := getLinuxDeviceInfo(name)
		if info == nil {
			info = &DeviceInfo{Path: path}
		} else {
			info = cloneLinuxDeviceInfo(info)
			info.Path = path
		}
		if infoErr != nil {
			infoErr = fmt.Errorf("read HID metadata for %s: %w", path, infoErr)
		}
		events = append(events, DeviceEvent{
			Type:       DeviceEventConnected,
			DeviceInfo: info,
			Err:        infoErr,
		})
	}
	return events, nil
}

func openLinuxUeventSocket() (socketFD int, wakeFD int, err error) {
	socketFD, err = unix.Socket(
		unix.AF_NETLINK,
		unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK,
		unix.NETLINK_KOBJECT_UEVENT,
	)
	if err != nil {
		return -1, -1, fmt.Errorf("open Linux uevent socket: %w", err)
	}

	// This is best effort: the kernel can clamp the requested size according to
	// net.core.rmem_max, but even a clamped increase reduces ENOBUFS risk.
	_ = unix.SetsockoptInt(socketFD, unix.SOL_SOCKET, unix.SO_RCVBUF, linuxUeventRcvbufSize)

	closeSocket := func() {
		_ = unix.Close(socketFD)
	}
	wakeFD, err = unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		closeSocket()
		return -1, -1, fmt.Errorf("open Linux HID event wake descriptor: %w", err)
	}
	if err := unix.Bind(socketFD, &unix.SockaddrNetlink{
		Family: unix.AF_NETLINK,
		Groups: linuxUeventGroup,
	}); err != nil {
		_ = unix.Close(wakeFD)
		closeSocket()
		return -1, -1, fmt.Errorf("bind Linux uevent socket: %w", err)
	}

	return socketFD, wakeFD, nil
}

// Events publishes a connected event for every currently enumerated HID device,
// followed by live connection and removal events.
func Events() (EventReceiver, error) {
	socketFD, wakeFD, err := openLinuxUeventSocket()
	if err != nil {
		return nil, err
	}

	receiver := &linuxEventReceiver{
		events:         newDeviceEventQueue(),
		socketFD:       socketFD,
		wakeFD:         wakeFD,
		stopped:        make(chan struct{}),
		loadDeviceInfo: getLinuxDeviceInfo,
		initializing:   true,
		devices:        make(map[string]*DeviceInfo),
	}
	go receiver.run()

	snapshot, err := linuxInitialEventSnapshot()
	if err != nil {
		closeErr := receiver.Close()
		return nil, errors.Join(fmt.Errorf("enumerate initial HID snapshot: %w", err), closeErr)
	}
	receiver.publishStartup(snapshot)

	receiver.mu.Lock()
	runErr := receiver.runErr
	receiver.mu.Unlock()
	if runErr != nil {
		return nil, receiver.Close()
	}

	return receiver, nil
}
