// Copyright © 2025 Johnnie Chen . All rights reserved.
// See accompanying LICENSE file

package supervisor

import (
	"os"
	"testing"
)

func TestRPCClient_Call(t *testing.T) {
	var client = NewRPCClient(os.Getenv("TEST_XML_RPC_URL"))
	{
		version, err := client.GetAPIVersion()
		t.Logf("GetAPIVersion: %s error=%v", version, err)
	}
	{
		pid, err := client.GetSupervisordPID()
		t.Logf("GetSupervisordPID: %d error=%v", pid, err)
	}
	{
		state, err := client.GetSupervisordState()
		t.Logf("GetSupervisordState: %v error=%v", state, err)
	}
	{
		infoList, err := client.GetAllProcessInfo()
		t.Logf("GetAllProcessInfo: error=%v", err)
		for _, info := range infoList {
			t.Logf("GetAllProcessInfo: info=%v", info)
		}
	}
}

func TestRPCClient_StartProcessGroup(t *testing.T) {
	var client = NewRPCClient(os.Getenv("TEST_XML_RPC_URL"))
	{
		info, err := client.GetProcessInfo("vr:center")
		t.Logf("GetProcessInfo: %v error=%v", info, err)
	}
	{
		err := client.StopProcess("vr:center")
		t.Logf("StopProcess: error=%v", err)
	}
	{
		err := client.StartProcess("vr:center")
		t.Logf("StartProcess: error=%v", err)
	}
	{
		infoList, err := client.StopProcessGroup("vr")
		t.Logf("StopProcessGroup: %v error=%v", infoList, err)
	}
	{
		infoList, err := client.StartProcessGroup("vr")
		t.Logf("StartProcessGroup: %v error=%v", infoList, err)
	}
	{
		infoList, err := client.stopAllProcesses()
		t.Logf("stopAllProcesses: %v error=%v", infoList, err)
	}
	{
		infoList, err := client.StartAllProcesses()
		t.Logf("StartAllProcesses: %v error=%v", infoList, err)
	}
}

func TestRPCClient_ReloadConfig(t *testing.T) {
	var client = NewRPCClient(os.Getenv("TEST_XML_RPC_URL"))
	{
		result, err := client.ReloadConfig()
		t.Logf("ReloadConfig: %v error=%v", result, err)
	}
}
