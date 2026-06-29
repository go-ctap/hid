package hid

import "os"

type Device struct {
	file *os.File
}
