// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package logger

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConsole(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "console.log"))
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Error(err)
		}
	})
	return f
}

func closeWriterAfterTest(t *testing.T, w *Writer) {
	t.Helper()
	t.Cleanup(func() { assert.NoError(t, w.Close()) })
}

func TestWriter_Write(t *testing.T) {
	for _, tc := range []struct {
		name    string
		console bool
		file    bool
	}{
		{"console", true, false},
		{"file", false, true},
		{"both", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var console *os.File
			if tc.console {
				console = testConsole(t)
			}
			var filename string
			if tc.file {
				filename = filepath.Join(t.TempDir(), "output.log")
			}
			w := NewWriter(filename, "", 100, 1)
			w.console = console
			closeWriterAfterTest(t, w)
			n, err := w.Write([]byte("hello"))
			require.NoError(t, err)
			assert.Equal(t, 5, n)
			if tc.console {
				data, err := os.ReadFile(console.Name())
				require.NoError(t, err)
				assert.Equal(t, "hello", string(data))
			}
			if tc.file {
				data, err := os.ReadFile(filename)
				require.NoError(t, err)
				assert.Equal(t, "hello", string(data))
			}
		})
	}
}

func TestWriter_WriteErrors(t *testing.T) {
	for _, tc := range []struct {
		name          string
		consoleFailed bool
		fileFailed    bool
	}{
		{"console failure still writes file", true, false},
		{"file failure still writes console", false, true},
		{"both failures are reported", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			console := testConsole(t)
			if tc.consoleFailed {
				require.NoError(t, console.Close())
			}
			filename := filepath.Join(t.TempDir(), "output.log")
			if tc.fileFailed {
				require.NoError(t, os.WriteFile(filename, []byte("not a directory"), 0644))
				filename = filepath.Join(filename, "output.log")
			}
			w := NewWriter(filename, "", 100, 1)
			w.console = console
			closeWriterAfterTest(t, w)
			n, err := w.Write([]byte("hello"))
			require.Error(t, err)
			assert.Zero(t, n)
			if tc.consoleFailed {
				assert.ErrorIs(t, err, os.ErrClosed)
				assert.ErrorContains(t, err, "write console")
			} else {
				data, readErr := os.ReadFile(console.Name())
				require.NoError(t, readErr)
				assert.Equal(t, "hello", string(data))
			}
			if tc.fileFailed {
				assert.ErrorContains(t, err, "write log file")
			} else {
				data, readErr := os.ReadFile(filename)
				require.NoError(t, readErr)
				assert.Equal(t, "hello", string(data))
			}
		})
	}
}

func TestWriter_NoOutput(t *testing.T) {
	for _, console := range []string{"", "unknown"} {
		w := NewWriter("", console, 100, 1)
		n, err := w.Write([]byte("hello"))
		assert.Zero(t, n)
		assert.ErrorContains(t, err, "no output configured")
		n, err = w.Write(nil)
		assert.Zero(t, n)
		assert.NoError(t, err)
	}
}

func TestNewWriterConsole(t *testing.T) {
	assert.Same(t, os.Stdout, NewWriter("", "stdout", 100, 1).console)
	assert.Same(t, os.Stderr, NewWriter("", "stderr", 100, 1).console)
}

func TestWriter_CloseAndSync(t *testing.T) {
	console := testConsole(t)
	filename := filepath.Join(t.TempDir(), "output.log")
	w := NewWriter(filename, "", 100, 1)
	w.console = console
	closeWriterAfterTest(t, w)
	_, err := w.Write([]byte("before"))
	require.NoError(t, err)
	require.NoError(t, w.Sync())
	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
	// Renaming and writing again verifies that Close released the old file;
	// lumberjack reopens the configured name on the next write.
	previous := filename + ".previous"
	require.NoError(t, os.Rename(filename, previous))
	_, err = w.Write([]byte("after"))
	require.NoError(t, err)
	data, err := os.ReadFile(previous)
	require.NoError(t, err)
	assert.Equal(t, "before", string(data))
	data, err = os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, "after", string(data))
	data, err = os.ReadFile(console.Name())
	require.NoError(t, err)
	assert.Equal(t, "beforeafter", string(data))
}

func TestWriter_Rotation(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "output.log")
	w := NewWriter(filename, "", 1, 1)
	closeWriterAfterTest(t, w)
	first := bytes.Repeat([]byte("a"), 700*1024)
	second := bytes.Repeat([]byte("b"), 700*1024)
	_, err := w.Write(first)
	require.NoError(t, err)
	_, err = w.Write(second)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(filename), "output-*.log"))
	require.NoError(t, err)
	require.Len(t, backups, 1)
	data, err := os.ReadFile(backups[0])
	require.NoError(t, err)
	assert.Equal(t, first, data)
	data, err = os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, second, data)
}

func TestWriter_ConcurrentWritesAndClose(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "output.log")
	w := NewWriter(filename, "", 100, 1)
	closeWriterAfterTest(t, w)
	const workers, writes = 8, 64
	errs := make(chan error, workers+1)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			for i := range writes {
				msg := fmt.Sprintf("%d:%d\n", worker, i)
				n, err := w.Write([]byte(msg))
				if err != nil {
					errs <- err
					return
				}
				if n != len(msg) {
					errs <- fmt.Errorf("short write: %d of %d", n, len(msg))
					return
				}
			}
		})
	}
	wg.Go(func() {
		for range writes {
			if err := w.Close(); err != nil {
				errs <- err
				return
			}
		}
	})
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	require.NoError(t, w.Close())
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	require.Len(t, lines, workers*writes)
	seen := make(map[string]bool, len(lines))
	for _, line := range lines {
		assert.False(t, seen[line], "duplicate entry: %q", line)
		seen[line] = true
	}
	for worker := range workers {
		for i := range writes {
			assert.True(t, seen[fmt.Sprintf("%d:%d", worker, i)])
		}
	}
}

func TestAppendFileLog(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "append.log")
	require.NoError(t, AppendFileLog(filename, "hello %d\n", 1))
	require.NoError(t, AppendFileLog(filename, "hello %d\n", 2))
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, "hello 1\nhello 2\n", string(data))
	assert.Error(t, AppendFileLog(t.TempDir(), "hello"))
}

func TestIsTerminal(t *testing.T) {
	assert.False(t, IsTerminal(testConsole(t)))
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, r.Close())
		assert.NoError(t, w.Close())
	})
	assert.False(t, IsTerminal(w))
	var sb strings.Builder
	assert.False(t, IsTerminal(&sb))
}
