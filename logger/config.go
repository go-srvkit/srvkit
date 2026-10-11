// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package logger

import (
	"errors"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const LayoutISO8601 = "2006-01-02T15:04:05.000Z0700"

// Config 配置参数. Build methods may be called concurrently if these fields are
// not modified. File outputs must have a single owner: do not construct multiple
// loggers writing to the same filename.
type Config struct {
	Level      string `json:"level"`       // DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL
	Encoding   string `json:"encoding"`    // "json" or "console"
	TimeLayout string `json:"time-layout"` // layout to encoding time
	Filename   string `json:"filename"`    //
	MaxSize    int    `json:"max_size"`    // defaults to 100 MB
	MaxBackups int    `json:"max_backups"` // maximum number of old log files to retain
	CallerSkip int    `json:"caller_skip"` // extra application stack frames; package wrappers are skipped automatically
}

func NewConfig() *Config {
	return &Config{
		Level:      "debug",
		Encoding:   "console",
		TimeLayout: LayoutISO8601,
		MaxSize:    100,
		MaxBackups: 10,
		CallerSkip: 0,
	}
}

func (c *Config) initDefault() {
	if c.Level == "" {
		c.Level = "INFO"
	}
	if c.Encoding == "" {
		c.Encoding = "console"
	}
	if c.TimeLayout == "" {
		c.TimeLayout = LayoutISO8601
	}
	if c.MaxSize <= 0 {
		c.MaxSize = 100
	}
	if c.MaxBackups <= 0 {
		c.MaxBackups = 10
	}
}

// Build constructs a logger and panics on invalid configuration. It does not
// modify c. Use BuildWithCleanup when file output needs an explicit shutdown.
func (c *Config) Build() *zap.Logger {
	log, _, err := c.BuildWithCleanup()
	if err != nil {
		panic(err)
	}
	return log
}

// BuildWithCleanup constructs a logger without modifying c and returns a
// cleanup function for its owned output. File output is opened lazily on write;
// I/O failures are reported through zap's error output. Stop all users of the
// logger and its children before cleanup. Cleanup may be called repeatedly and
// does not close stdout. Sync flushes no buffers and does not guarantee fsync.
// Lumberjack retains its maintenance goroutine after closing a file; reuse
// long-lived loggers rather than repeatedly constructing file outputs.
func (c *Config) BuildWithCleanup() (*zap.Logger, func() error, error) {
	cfg, level, err := normalizeConfig(c)
	if err != nil {
		return nil, nil, err
	}
	core, closeOutput := createZapCore(cfg, level, os.Stdout, IsTerminal(os.Stdout))
	log := zap.New(core,
		zap.AddCallerSkip(cfg.CallerSkip),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.PanicLevel))
	return log, func() error { return errors.Join(log.Sync(), closeOutput()) }, nil
}

// CreateZapCore constructs a core with the same defaults as Build and panics on
// invalid configuration. It does not modify c. For file output with an explicit
// shutdown, use BuildWithCleanup instead.
func CreateZapCore(c *Config) zapcore.Core {
	cfg, level, err := normalizeConfig(c)
	if err != nil {
		panic(err)
	}
	core, _ := createZapCore(cfg, level, os.Stdout, IsTerminal(os.Stdout))
	return core
}

func normalizeConfig(c *Config) (Config, zapcore.Level, error) {
	if c == nil {
		return Config{}, 0, errors.New("logger: nil configuration")
	}
	cfg := *c
	cfg.initDefault()
	var level zapcore.Level
	if err := level.Set(cfg.Level); err != nil {
		return Config{}, 0, fmt.Errorf("logger level: %w", err)
	}
	if cfg.Encoding != "json" && cfg.Encoding != "console" {
		return Config{}, 0, fmt.Errorf("logger: unsupported encoding %q", cfg.Encoding)
	}
	if cfg.CallerSkip < 0 {
		return Config{}, 0, errors.New("logger: caller_skip must not be negative")
	}
	return cfg, level, nil
}

func createZapCore(c Config, level zapcore.Level, console *os.File, terminal bool) (zapcore.Core, func() error) {
	var encoderConfig = zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(c.TimeLayout),
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	newCore := func(w *Writer, color bool) zapcore.Core {
		cfg := encoderConfig
		if color {
			cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}
		var enc zapcore.Encoder
		if c.Encoding == "json" {
			enc = zapcore.NewJSONEncoder(cfg)
		} else {
			enc = zapcore.NewConsoleEncoder(cfg)
		}
		return zapcore.NewCore(enc, w, level)
	}
	color := c.Encoding == "console" && terminal
	w := NewWriter(c.Filename, "", c.MaxSize, c.MaxBackups)
	if !color || c.Filename == "" {
		// Identical output formats share a single encoding of each entry.
		w.console = console
		return newCore(w, color), w.Close
	}
	consoleWriter := NewWriter("", "", c.MaxSize, c.MaxBackups)
	consoleWriter.console = console
	return zapcore.NewTee(newCore(consoleWriter, true), newCore(w, false)), w.Close
}
