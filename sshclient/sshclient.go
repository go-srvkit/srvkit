// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package sshclient

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/melbahja/goph"
	"gopkg.in/srvkit.v1/gutil"

	"gopkg.in/srvkit.v1/logger"
)

var errSSHKeyNotFound = fmt.Errorf("ssh key file not found")

type StreamLogAppender interface {
	AppendLogf(format string, args ...any)
}

type singleWriter struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func (w *singleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func getDefaultSSHKeyFile() (string, error) {
	dirname, er := os.UserHomeDir()
	if er != nil {
		return "", er
	}
	var defaultKeyFile = filepath.Join(dirname, ".ssh/id_rsa")
	if gutil.IsFileExist(defaultKeyFile) {
		return defaultKeyFile, nil
	}
	if runtime.GOOS == "darwin" {
		defaultKeyFile = filepath.Join(dirname, ".ssh/id_ed25519")
		if gutil.IsFileExist(defaultKeyFile) {
			return defaultKeyFile, nil
		}
	}
	return "", errSSHKeyNotFound
}

func createSSHClient(host string, port uint16, user, pass string) (*goph.Client, error) {
	var err error
	var auth goph.Auth
	if pass == "" {
		keyFile, err := getDefaultSSHKeyFile()
		if err != nil {
			return nil, err
		}
		auth, err = goph.Key(keyFile, "")
		if err != nil {
			return nil, err
		}
	} else {
		data, er := base64.StdEncoding.DecodeString(pass)
		if er != nil {
			return nil, er
		}
		var pass = unsafe.String(unsafe.SliceData(data), len(data))
		auth = goph.Password(pass)
	}

	callback, err := goph.DefaultKnownHosts()
	if err != nil {
		return nil, err
	}
	return goph.NewConn(&goph.Config{
		User:     user,
		Addr:     host,
		Port:     uint(port),
		Auth:     auth,
		Timeout:  time.Second * 3,
		Callback: callback,
	})
}

func RunSSHCommand(host string, port uint16, user, pass, cmd string) ([]byte, error) {
	if host == "" || user == "" {
		return nil, fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass)
	if err != nil {
		logger.Errorf("create ssh client %s:%d failed: %v", host, port, err)
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	var buf singleWriter
	session.Stdout = &buf
	session.Stderr = &buf

	logger.Infof("start run remote ssh command: [%s] host=%s", cmd, host)

	if err = session.Run(cmd); err != nil {
		return buf.b.Bytes(), err
	}
	return buf.b.Bytes(), nil
}

func RunSSHCommandStream(host string, port uint16, user, pass, cmd string, cc StreamLogAppender) error {
	if host == "" || user == "" {
		return fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// stdout/stderr pipe
	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}
	logger.Infof("start run remote ssh command: [%s] host=%s", cmd, host)

	if err = session.Start(cmd); err != nil {
		return err
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			cc.AppendLogf(scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			cc.AppendLogf(scanner.Text())
		}
	}()
	return session.Wait()
}

func ReadSSHFile(host string, port uint16, user, pass, fullpath string) ([]byte, error) {
	if host == "" || user == "" {
		return nil, fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sftp, err := client.NewSftp()
	if err != nil {
		return nil, err
	}
	defer sftp.Close()
	f, err := sftp.OpenFile(fullpath, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func WriteSSHFile(host string, port uint16, user, pass, fullpath string, data []byte) error {
	if host == "" || user == "" {
		return fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass)
	if err != nil {
		return err
	}
	defer client.Close()

	sftp, err := client.NewSftp()
	if err != nil {
		return err
	}
	defer sftp.Close()
	f, err := sftp.OpenFile(fullpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
