// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.
package sshclient

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	sshPort        uint16 = 22
	defaultTimeout        = time.Second * 5
)

type FakeStream struct {
}

func (f *FakeStream) AppendLogf(format string, args ...any) {
}

func TestRunSSHCommandSimple(t *testing.T) {
	output, err := RunSSHCommand("10.23.0.76", 12122, "devel", "", defaultTimeout, "cat /proc/cpuinfo")
	assert.Equal(t, nil, err)
	t.Logf("output: %s", output)
}

func TestRunSSHCommand(t *testing.T) {
	var sshHost = os.Getenv("TEST_SSH_HOST")
	if sshHost != "" {
		var sshUser = os.Getenv("TEST_SSH_USER")
		var sshPass = os.Getenv("TEST_SSH_PASS")
		output, err := RunSSHCommand(sshHost, sshPort, sshUser, sshPass, defaultTimeout, "cat /proc/cpuinfo")
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
		err := RunSSHCommandStream(sshHost, sshPort, sshUser, sshPass, defaultTimeout, "cat /proc/cpuinfo", stream)
		assert.Equal(t, nil, err)
	}
}

func TestReadSSHFile(t *testing.T) {
	var sshHost = os.Getenv("TEST_SSH_HOST")
	if sshHost != "" {
		var sshUser = os.Getenv("TEST_SSH_USER")
		var sshPass = os.Getenv("TEST_SSH_PASS")
		output, err := ReadSSHFile(sshHost, sshPort, sshUser, sshPass, defaultTimeout, "/etc/supervisord.conf")
		assert.Equal(t, nil, err)
		t.Logf("output: %s", output)
	}
}
