// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Tests replacing stdout must not run in parallel. The built core keeps its
// own console reference, so restore stdout before returning to the caller.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	console := testConsole(t)
	previous := os.Stdout
	os.Stdout = console
	defer func() { os.Stdout = previous }()
	fn()
	data, err := os.ReadFile(console.Name())
	require.NoError(t, err)
	return data
}

func TestConfig_Build(t *testing.T) {
	for _, encoding := range []string{"console", "json"} {
		t.Run(encoding, func(t *testing.T) {
			conf := NewConfig()
			conf.Encoding = encoding
			conf.TimeLayout = "literal"
			conf.Level = "warn"
			before := *conf
			data := captureStdout(t, func() {
				log := conf.Build()
				assert.Equal(t, zapcore.WarnLevel, log.Level())
				log.Info("filtered")
				log.Warn("hello", zap.String("key", "value"))
				require.NoError(t, log.Sync())
			})
			assert.Equal(t, before, *conf)
			assert.NotContains(t, string(data), "filtered")
			if encoding == "json" {
				var entry map[string]interface{}
				require.NoError(t, json.Unmarshal(data, &entry))
				assert.Equal(t, "literal", entry["time"])
				assert.Equal(t, "WARN", entry["level"])
				assert.Equal(t, "hello", entry["msg"])
				assert.Equal(t, "value", entry["key"])
			} else {
				assert.Contains(t, string(data), "literal\tWARN\t")
				assert.Contains(t, string(data), "hello")
				assert.Contains(t, string(data), `"key": "value"`)
			}
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	for _, buildCore := range []bool{false, true} {
		name := "Build"
		if buildCore {
			name = "CreateZapCore"
		}
		t.Run(name, func(t *testing.T) {
			conf := &Config{Encoding: "json"}
			before := *conf
			data := captureStdout(t, func() {
				var log *zap.Logger
				if buildCore {
					log = zap.New(CreateZapCore(conf))
				} else {
					log = conf.Build()
				}
				assert.Equal(t, zapcore.InfoLevel, log.Level())
				log.Info("defaults")
			})
			assert.Equal(t, before, *conf)
			var entry map[string]interface{}
			require.NoError(t, json.Unmarshal(data, &entry))
			timestamp, ok := entry["time"].(string)
			require.True(t, ok)
			_, err := time.Parse(LayoutISO8601, timestamp)
			assert.NoError(t, err)
		})
	}
	conf, _, err := normalizeConfig(&Config{})
	require.NoError(t, err)
	assert.Equal(t, 100, conf.MaxSize)
	assert.Equal(t, 10, conf.MaxBackups)
	assert.Equal(t, "console", conf.Encoding)
}

func TestConfig_Invalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		conf *Config
		want string
	}{
		{"nil", nil, "nil configuration"},
		{"level", &Config{Level: "infp"}, "logger level"},
		{"encoding", &Config{Encoding: "yaml"}, "unsupported encoding"},
		{"caller skip", &Config{CallerSkip: -1}, "caller_skip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log, cleanup, err := tc.conf.BuildWithCleanup()
			assert.Nil(t, log)
			assert.Nil(t, cleanup)
			assert.ErrorContains(t, err, tc.want)
			assert.Panics(t, func() { tc.conf.Build() })
			assert.Panics(t, func() { CreateZapCore(tc.conf) })
		})
	}
}

func TestConfig_BuildWithCleanup(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "output.log")
	conf := &Config{Filename: filename, Encoding: "json"}
	data := captureStdout(t, func() {
		log, cleanup, err := conf.BuildWithCleanup()
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, cleanup()) })
		log.With(zap.String("child", "yes")).Info("hello")
		require.NoError(t, cleanup())
		require.NoError(t, cleanup())
		// On Windows this also verifies that the file handle was closed.
		require.NoError(t, os.Rename(filename, filename+".closed"))
		_, err = os.Stdout.Write([]byte("console remains open\n"))
		require.NoError(t, err)
	})
	fileData, err := os.ReadFile(filename + ".closed")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(data), string(fileData)))
	assert.Contains(t, string(data), "console remains open\n")
	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(fileData, &entry))
	assert.Equal(t, "yes", entry["child"])
}

func TestConfig_OutputColors(t *testing.T) {
	for _, tc := range []struct {
		encoding string
		terminal bool
		color    bool
	}{
		{"console", true, true},
		{"console", false, false},
		{"json", true, false},
	} {
		name := tc.encoding + "/redirected"
		if tc.terminal {
			name = tc.encoding + "/terminal"
		}
		t.Run(name, func(t *testing.T) {
			console := testConsole(t)
			filename := filepath.Join(t.TempDir(), "output.log")
			conf, level, err := normalizeConfig(&Config{Encoding: tc.encoding, Filename: filename})
			require.NoError(t, err)
			core, cleanup := createZapCore(conf, level, console, tc.terminal)
			t.Cleanup(func() { assert.NoError(t, cleanup()) })
			zap.New(core).Info("hello", zap.String("key", "value"))
			fileData, err := os.ReadFile(filename)
			require.NoError(t, err)
			consoleData, err := os.ReadFile(console.Name())
			require.NoError(t, err)
			assert.NotContains(t, string(fileData), "\x1b[")
			assert.Equal(t, tc.color, strings.Contains(string(consoleData), "\x1b["))
			if !tc.color {
				assert.Equal(t, fileData, consoleData)
			}
			assert.Contains(t, string(fileData), "hello")
		})
	}
}

func TestConfig_ConcurrentBuild(t *testing.T) {
	var conf Config
	const workers = 8
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			_, cleanup, err := conf.BuildWithCleanup()
			if err == nil {
				err = cleanup()
			}
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err)
	}
	assert.Equal(t, Config{}, conf)
}

func TestConfig_FileWriteFailure(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(filename, []byte("not a directory"), 0644))
	conf := &Config{Encoding: "json", Filename: filepath.Join(filename, "output.log")}
	var diagnostics bytes.Buffer
	data := captureStdout(t, func() {
		log, cleanup, err := conf.BuildWithCleanup()
		require.NoError(t, err) // File opening is deliberately lazy.
		t.Cleanup(func() { assert.NoError(t, cleanup()) })
		log = log.WithOptions(zap.ErrorOutput(zapcore.AddSync(&diagnostics)))
		log.Info("console survives file failure")
	})
	assert.Contains(t, string(data), "console survives file failure")
	assert.Contains(t, diagnostics.String(), "write log file")
}
