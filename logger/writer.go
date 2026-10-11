// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package logger

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Writer writes logs to a file and/or console. Writes and Close may be called
// concurrently, provided its configuration fields are not modified after use.
// The console is borrowed and is never closed by Writer.
type Writer struct {
	lumberjack.Logger

	console *os.File
}

func NewWriter(filename, console string, maxSize, maxBackup int) *Writer {
	w := &Writer{
		Logger: lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize,
			MaxBackups: maxBackup,
			LocalTime:  true,
		},
	}
	switch console {
	case "stderr":
		w.console = os.Stderr
	case "stdout":
		w.console = os.Stdout
	}
	return w
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.console == nil && w.Filename == "" {
		if len(p) == 0 {
			return 0, nil
		}
		return 0, errors.New("logger: no output configured")
	}
	n = len(p)
	if w.console != nil {
		n, err = w.console.Write(p)
		if n < len(p) && err == nil {
			err = io.ErrShortWrite
		}
		if err != nil {
			err = fmt.Errorf("write console: %w", err)
		}
	}
	if w.Filename != "" {
		fileN, fileErr := w.Logger.Write(p)
		n = min(n, fileN)
		if fileN < len(p) && fileErr == nil {
			fileErr = io.ErrShortWrite
		}
		if fileErr != nil {
			err = errors.Join(err, fmt.Errorf("write log file: %w", fileErr))
		}
	}
	return
}

// Sync is a no-op because both outputs are unbuffered. It does not provide an
// fsync durability guarantee. Call Close to release the owned file output.
func (w *Writer) Sync() error {
	return nil
}

// Close closes the current log file, leaving the console open. Repeated calls
// are safe. A subsequent Write may reopen the file, as with lumberjack.Logger.
func (w *Writer) Close() error {
	return w.Logger.Close()
}

// AppendFileLog append log to file
func AppendFileLog(filename, format string, a ...interface{}) (err error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	_, err = fmt.Fprintf(f, format, a...)
	return err
}

func IsTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}
