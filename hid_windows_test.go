package hid

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestGetHidGuid(t *testing.T) {
	guid, err := getHidGuid()
	if err != nil {
		t.Fatal(err)
	}

	if guid == nil {
		t.Fatal("getHidGuid returned nil")
	}
	if *guid == (windows.GUID{}) {
		t.Fatal("getHidGuid returned an empty GUID")
	}
}
