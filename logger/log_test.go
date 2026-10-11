// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package logger

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func restoreLoggerAfterTest(t *testing.T) {
	t.Helper()
	previous := Logger()
	t.Cleanup(func() { SetLogger(previous) })
}

func TestLogger(t *testing.T) {
	assert.NotNil(t, Logger())
	assert.NotNil(t, Sugared())
	assert.Equal(t, zapcore.DebugLevel, Level())
}

func TestSetLogger(t *testing.T) {
	restoreLoggerAfterTest(t)
	core, logs := observer.New(zapcore.WarnLevel)
	log := zap.New(core)
	SetLogger(log)
	assert.Same(t, log, Logger())
	assert.Equal(t, zapcore.WarnLevel, Level())
	Info("filtered")
	Warnf("hello %s", "world")
	Sugared().Warnw("direct", "id", 42)
	entries := logs.All()
	require.Len(t, entries, 2)
	assert.Equal(t, "hello world", entries[0].Message)
	assert.Equal(t, int64(42), entries[1].ContextMap()["id"])
}

func TestSetLoggerWith(t *testing.T) {
	restoreLoggerAfterTest(t)
	core, logs := observer.New(zapcore.InfoLevel)
	SetLoggerWith(core, 0)
	assert.Equal(t, zapcore.InfoLevel, Level())
	Infof("hello %s", "world")
	entries := logs.All()
	require.Len(t, entries, 1)
	assert.Equal(t, "hello world", entries[0].Message)
}

func requireCaller(t *testing.T, logs *observer.ObservedLogs, file string, line int) {
	t.Helper()
	entries := logs.TakeAll()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Caller.Defined)
	assert.Equal(t, file, entries[0].Caller.File)
	assert.Equal(t, line, entries[0].Caller.Line)
}

func TestCaller(t *testing.T) {
	for _, setup := range []string{"Build", "SetLogger", "SetLoggerWith"} {
		t.Run(setup, func(t *testing.T) {
			restoreLoggerAfterTest(t)
			core, logs := observer.New(zapcore.DebugLevel)
			var log *zap.Logger
			switch setup {
			case "Build":
				log = NewConfig().Build().WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core { return core }))
				SetLogger(log)
			case "SetLogger":
				log = zap.New(core, zap.AddCaller())
				SetLogger(log)
			case "SetLoggerWith":
				SetLoggerWith(core, 0)
				log = Logger()
			}
			_, file, line, _ := runtime.Caller(0)
			log.Info("direct")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Logger().Info("accessor")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Sugared().Info("sugared accessor")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			With(zap.String("key", "value")).Info("child")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Info("package wrapper")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Infof("package %s", "format")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Infow("package fields", "id", 42)
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Infoln("package line")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Debug("package debug")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Warn("package warn")
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			Error("package error")
			requireCaller(t, logs, file, line+1)
		})
	}
}

func logThroughApplicationWrapper(log *zap.Logger) {
	log.Info("application wrapper")
}

func logThroughPackageWrapper() {
	Info("application and package wrappers")
}

func TestCaller_ExtraApplicationFrame(t *testing.T) {
	for _, setup := range []string{"Build", "SetLoggerWith"} {
		t.Run(setup, func(t *testing.T) {
			restoreLoggerAfterTest(t)
			core, logs := observer.New(zapcore.InfoLevel)
			if setup == "Build" {
				conf := NewConfig()
				conf.CallerSkip = 1
				SetLogger(conf.Build().WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core { return core })))
			} else {
				SetLoggerWith(core, 1)
			}
			_, file, line, _ := runtime.Caller(0)
			logThroughApplicationWrapper(Logger())
			requireCaller(t, logs, file, line+1)
			_, file, line, _ = runtime.Caller(0)
			logThroughPackageWrapper()
			requireCaller(t, logs, file, line+1)
		})
	}
}

type syncRecorder struct {
	writes int
	syncs  int
	err    error
}

func (w *syncRecorder) Write(p []byte) (int, error) {
	w.writes++
	return len(p), nil
}

func (w *syncRecorder) Sync() error {
	w.syncs++
	return w.err
}

func TestSync(t *testing.T) {
	restoreLoggerAfterTest(t)
	w := &syncRecorder{}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), w, zapcore.InfoLevel)
	SetLogger(zap.New(core))
	Infof("hello")
	assert.NoError(t, Sync())
	assert.Equal(t, 1, w.writes)
	assert.Equal(t, 1, w.syncs)
	w.err = errors.New("sync failure")
	assert.ErrorIs(t, Sync(), w.err)
	assert.Equal(t, 2, w.syncs)
}
