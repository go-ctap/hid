package hid

import (
	"os"
	"sync"
)

type Device struct {
	file    *os.File
	readMu  sync.Mutex
	writeMu sync.Mutex
}
