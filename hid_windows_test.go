package hid

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestGetHidGuid(t *testing.T) {
	guid, err := getHidGuid()
	require.NoError(t, err)

	require.NotNil(t, guid)
	require.NotEmpty(t, guid)
}

func TestWithOpenAccess(t *testing.T) {
	d := &Device{}
	WithOpenAccess(windows.GENERIC_WRITE)(d)

	require.Equal(t, uint32(windows.GENERIC_WRITE), d.openAccess)
}
