// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package logger

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Writer write log to file and console
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
	if w.console != nil {
		n, err = w.console.Write(p)
	}
	if w.Filename != "" {
		n, err = w.Logger.Write(p)
	}
	return
}

// AppendFileLog append log to file
func AppendFileLog(filename, format string, a ...interface{}) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
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
