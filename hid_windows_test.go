package hid

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetHidGuid(t *testing.T) {
	guid, err := getHidGuid()
	require.NoError(t, err)

	require.NotNil(t, guid)
	require.NotEmpty(t, guid)
}
