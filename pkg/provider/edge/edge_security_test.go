package edge

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type noopWriteCloser struct{}

func (noopWriteCloser) Write(p []byte) (n int, err error) { return len(p), nil }
func (noopWriteCloser) Close() error                      { return nil }

func TestNewWithClient_DoesNotCreateTapeFileByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})
	require.NoError(t, os.Chdir(tmpDir))

	cl, err := NewWithClient("workspace", "T123", "xoxp-secret", &http.Client{})
	require.NoError(t, err)
	require.NotNil(t, cl)

	_, err = os.Stat(filepath.Join(tmpDir, "tape.txt"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestNewWithClient_AppliesWithTapeOption(t *testing.T) {
	expectedTape := noopWriteCloser{}

	cl, err := NewWithClient("workspace", "T123", "xoxp-secret", &http.Client{}, WithTape(expectedTape))
	require.NoError(t, err)
	require.NotNil(t, cl)

	require.IsType(t, expectedTape, cl.tape)
	_, ok := cl.tape.(io.WriteCloser)
	require.True(t, ok)
}
