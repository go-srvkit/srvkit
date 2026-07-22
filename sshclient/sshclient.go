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
	"sync"
	"time"

	"github.com/melbahja/goph/v2"
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
	candidates := []string{"id_ed25519", "id_rsa", "id_ecdsa", "id_dsa"}
	for _, name := range candidates {
		p := filepath.Join(dirname, ".ssh", name)
		if gutil.IsFileExist(p) {
			return p, nil
		}
	}
	return "", errSSHKeyNotFound
}

// `pass` is base64-encoded raw cipher text
func createSSHClient(host string, port uint16, user, pass string, timeout time.Duration) (*goph.Client, error) {
	var opts []goph.Option
	if timeout > 0 {
		opts = append(opts, goph.WithTimeout(timeout))
	}
	if pass != "" {
		data, er := base64.StdEncoding.DecodeString(pass)
		if er != nil {
			return nil, er
		}
		opts = append(opts, goph.WithPassword(string(data)))
	} else {
		keyFile, err := getDefaultSSHKeyFile()
		if err != nil {
			return nil, err
		}
		opts = append(opts, goph.WithKeyFile(keyFile, ""))
	}
	{
		if homedir, er := os.UserHomeDir(); er == nil {
			var filename = filepath.Join(homedir, ".ssh/known_hosts")
			if gutil.IsFileExist(filename) {
				opts = append(opts, goph.WithKnownHosts(filename))
			}
		}
	}
	return goph.New(user, host, opts...)
}

// RunSSHCommand run ssh command
func RunSSHCommand(host string, port uint16, user, pass string, timeout time.Duration, cmd string) ([]byte, error) {
	if host == "" || user == "" {
		return nil, fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass, timeout)
	if err != nil {
		logger.Errorf("create ssh client %s:%d failed: %v", host, port, err)
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var buf singleWriter
	session.Stdout = &buf
	session.Stderr = &buf

	logger.Infof("start run remote ssh command: [%s] host=%s", cmd, host)

	if err = session.Run(cmd); err != nil {
		return buf.b.Bytes(), err
	}
	return buf.b.Bytes(), nil
}

// RunSSHCommandStream run ssh command and write to log to stream
func RunSSHCommandStream(host string, port uint16, user, pass string, timeout time.Duration, cmd string, cc StreamLogAppender) error {
	if host == "" || user == "" {
		return fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass, timeout)
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

// ReadSSHFile read file from remote server
func ReadSSHFile(host string, port uint16, user, pass string, timeout time.Duration, fullpath string) ([]byte, error) {
	if host == "" || user == "" {
		return nil, fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass, timeout)
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

// WriteSSHFile write file to remote server
func WriteSSHFile(host string, port uint16, user, pass string, timeout time.Duration, fullpath string, data []byte) error {
	if host == "" || user == "" {
		return fmt.Errorf("ssh host or user is empty")
	}
	client, err := createSSHClient(host, port, user, pass, timeout)
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
