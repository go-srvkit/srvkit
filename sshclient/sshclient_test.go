// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.
package sshclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	sshPort uint16 = 22
)

type FakeStream struct {
}

func (f *FakeStream) AppendLogf(format string, args ...any) {
}

func TestRunSSHCommand(t *testing.T) {
	var sshHost = os.Getenv("TEST_SSH_HOST")
	if sshHost != "" {
		var sshUser = os.Getenv("TEST_SSH_USER")
		var sshPass = os.Getenv("TEST_SSH_PASS")
		output, err := RunSSHCommand(sshHost, sshPort, sshUser, sshPass, "cat /proc/cpuinfo")
		assert.Equal(t, nil, err)
		t.Logf("output: %s", output)
	}
}

func TestRunSSHCommandStream(t *testing.T) {
	var sshHost = os.Getenv("TEST_SSH_HOST")
	if sshHost != "" {
		var sshUser = os.Getenv("TEST_SSH_USER")
		var sshPass = os.Getenv("TEST_SSH_PASS")
		var stream = new(FakeStream)
		err := RunSSHCommandStream(sshHost, sshPort, sshUser, sshPass, "cat /proc/cpuinfo", stream)
		assert.Equal(t, nil, err)
	}
}

func TestReadSSHFile(t *testing.T) {
	var sshHost = os.Getenv("TEST_SSH_HOST")
	if sshHost != "" {
		var sshUser = os.Getenv("TEST_SSH_USER")
		var sshPass = os.Getenv("TEST_SSH_PASS")
		output, err := ReadSSHFile(sshHost, sshPort, sshUser, sshPass, "/etc/supervisord.conf")
		assert.Equal(t, nil, err)
		t.Logf("output: %s", output)
	}
}
