// Copyright © 2025 Johnnie Chen . All rights reserved.
// See accompanying LICENSE file

package supervisor

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type rpcTestStep struct {
	method   string
	args     []string
	response string
	status   int
}

// Use real XML-RPC requests and responses so method names, parameter ordering,
// XML escaping, and response decoding are checked together with the wrappers.
func newScriptedRPCClient(t *testing.T, steps ...rpcTestStep) *RPCClient {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		index := calls
		calls++
		mu.Unlock()
		if index >= len(steps) {
			t.Errorf("unexpected RPC request %d", index+1)
			http.Error(w, "unexpected RPC request", http.StatusInternalServerError)
			return
		}
		step := steps[index]
		if r.Method != http.MethodPost || r.URL.Path != "/RPC2" {
			t.Errorf("request = %s %s, want POST /RPC2", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "text/xml" {
			t.Errorf("Content-Type = %q, want text/xml", got)
		}
		var request struct {
			Method string `xml:"methodName"`
			Params []struct {
				Value struct {
					String string `xml:"string"`
				} `xml:"value"`
			} `xml:"params>param"`
		}
		if err := xml.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		var args []string
		for _, param := range request.Params {
			args = append(args, param.Value.String)
		}
		if request.Method != step.method || !reflect.DeepEqual(args, step.args) {
			t.Errorf("RPC request %d = %s(%q), want %s(%q)", index+1, request.Method, args, step.method, step.args)
		}
		w.Header().Set("Content-Type", "text/xml")
		if step.status != 0 {
			w.WriteHeader(step.status)
		}
		io.WriteString(w, step.response)
	}))
	client := NewRPCClient(server.URL + "/RPC2")
	if client == nil {
		server.Close()
		t.Fatal("NewRPCClient returned nil for a valid URL")
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
		server.Close()
		mu.Lock()
		defer mu.Unlock()
		if calls != len(steps) {
			t.Errorf("RPC calls = %d, want %d", calls, len(steps))
		}
	})
	return client
}

func rpcTestString(value string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(value))
	return "<value><string>" + buf.String() + "</string></value>"
}

func rpcTestArray(values ...string) string {
	return "<value><array><data>" + strings.Join(values, "") + "</data></array></value>"
}

func rpcTestResponse(value string) string {
	return "<methodResponse><params><param>" + value + "</param></params></methodResponse>"
}

func rpcTestReload(added, changed, removed []string) string {
	groups := make([]string, 0, 3)
	for _, names := range [][]string{added, changed, removed} {
		values := make([]string, 0, len(names))
		for _, name := range names {
			values = append(values, rpcTestString(name))
		}
		groups = append(groups, rpcTestArray(values...))
	}
	return rpcTestResponse(rpcTestArray(rpcTestArray(groups...)))
}

const rpcTestTrue = "<value><boolean>1</boolean></value>"
const rpcTestFalse = "<value><boolean>0</boolean></value>"
const rpcTestFault = `<methodResponse><fault><value><struct>
<member><name>faultCode</name><value><int>10</int></value></member>
<member><name>faultString</name><value><string>BAD_NAME: missing process</string></value></member>
</struct></value></fault></methodResponse>`

const rpcTestProcessValue = `<value><struct>
<member><name>name</name><value><string>worker</string></value></member>
<member><name>group</name><value><string>services</string></value></member>
<member><name>description</name><value><string>pid 123, uptime 0:01:00</string></value></member>
<member><name>pid</name><value><int>123</int></value></member>
<member><name>start</name><value><int>1700000000</int></value></member>
<member><name>stop</name><value><int>0</int></value></member>
<member><name>now</name><value><int>1700000060</int></value></member>
<member><name>state</name><value><int>20</int></value></member>
<member><name>statename</name><value><string>RUNNING</string></value></member>
<member><name>spawnerr</name><value><string></string></value></member>
<member><name>exitstatus</name><value><int>0</int></value></member>
<member><name>logfile</name><value><string>/tmp/worker.log</string></value></member>
<member><name>stdout_logfile</name><value><string>/tmp/worker.log</string></value></member>
<member><name>stderr_logfile</name><value><string>/tmp/worker.err</string></value></member>
</struct></value>`

const rpcTestStatusValue = `<value><struct>
<member><name>name</name><value><string>worker</string></value></member>
<member><name>group</name><value><string>services</string></value></member>
<member><name>description</name><value><string>OK</string></value></member>
<member><name>status</name><value><int>80</int></value></member>
</struct></value>`

func rpcTestProcessInfo() ProcessInfo {
	return ProcessInfo{
		Name: "worker", Group: "services", Description: "pid 123, uptime 0:01:00",
		Pid: 123, Start: 1700000000, Now: 1700000060, State: 20, Statename: "RUNNING",
		Logfile: "/tmp/worker.log", StdoutLogfile: "/tmp/worker.log", StderrLogfile: "/tmp/worker.err",
	}
}

type rpcTestMethod struct {
	method string
	args   []string
	value  string
	want   any
	call   func(*RPCClient) (any, error)
}

func rpcTestMethods() []rpcTestMethod {
	// Metacharacters ensure named operations XML-escape their argument.
	const name = "services:worker<&>"
	statuses := []ProcessStatusInfo{{Name: "worker", Group: "services", Description: "OK", Status: 80}}
	return []rpcTestMethod{
		{"supervisor.getAPIVersion", nil, rpcTestString("3.0"), "3.0", func(c *RPCClient) (any, error) { return c.GetAPIVersion() }},
		{"supervisor.getPID", nil, "<value><int>456</int></value>", 456, func(c *RPCClient) (any, error) { return c.GetSupervisordPID() }},
		{"supervisor.getState", nil, `<value><struct><member><name>statecode</name><value><int>1</int></value></member><member><name>statename</name><value><string>RUNNING</string></value></member></struct></value>`, ServerState{Statecode: 1, Statename: "RUNNING"}, func(c *RPCClient) (any, error) { return c.GetSupervisordState() }},
		{"supervisor.getAllProcessInfo", nil, rpcTestArray(rpcTestProcessValue), []ProcessInfo{rpcTestProcessInfo()}, func(c *RPCClient) (any, error) { return c.GetAllProcessInfo() }},
		{"supervisor.getProcessInfo", []string{name}, rpcTestProcessValue, rpcTestProcessInfo(), func(c *RPCClient) (any, error) { return c.GetProcessInfo(name) }},
		{"supervisor.stopProcess", []string{name}, rpcTestTrue, nil, func(c *RPCClient) (any, error) { return nil, c.StopProcess(name) }},
		{"supervisor.startProcess", []string{name}, rpcTestTrue, nil, func(c *RPCClient) (any, error) { return nil, c.StartProcess(name) }},
		{"supervisor.stopProcessGroup", []string{name}, rpcTestArray(rpcTestStatusValue), statuses, func(c *RPCClient) (any, error) { return c.StopProcessGroup(name) }},
		{"supervisor.startProcessGroup", []string{name}, rpcTestArray(rpcTestStatusValue), statuses, func(c *RPCClient) (any, error) { return c.StartProcessGroup(name) }},
		{"supervisor.stopAllProcesses", nil, rpcTestArray(rpcTestStatusValue), statuses, func(c *RPCClient) (any, error) { return c.stopAllProcesses() }},
		{"supervisor.startAllProcesses", nil, rpcTestArray(rpcTestStatusValue), statuses, func(c *RPCClient) (any, error) { return c.StartAllProcesses() }},
		{"supervisor.reloadConfig", nil, rpcTestArray(rpcTestArray(rpcTestArray(rpcTestString("new")), rpcTestArray(rpcTestString("changed")), rpcTestArray(rpcTestString("old")))), ReloadConfigResult{Added: []string{"new"}, Changed: []string{"changed"}, Removed: []string{"old"}}, func(c *RPCClient) (any, error) { return c.ReloadConfig() }},
		{"supervisor.addProcessGroup", []string{name}, rpcTestTrue, nil, func(c *RPCClient) (any, error) { return nil, c.AddProcessGroup(name) }},
		{"supervisor.removeProcessGroup", []string{name}, rpcTestTrue, nil, func(c *RPCClient) (any, error) { return nil, c.RemoveProcessGroup(name) }},
	}
}

func TestRPCClientMethods(t *testing.T) {
	for _, tt := range rpcTestMethods() {
		t.Run(tt.method, func(t *testing.T) {
			client := newScriptedRPCClient(t, rpcTestStep{method: tt.method, args: tt.args, response: rpcTestResponse(tt.value)})
			got, err := tt.call(client)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("result = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRPCClientMethodFaults(t *testing.T) {
	for _, tt := range rpcTestMethods() {
		t.Run(tt.method, func(t *testing.T) {
			client := newScriptedRPCClient(t, rpcTestStep{method: tt.method, args: tt.args, response: rpcTestFault})
			_, err := tt.call(client)
			if err == nil || !strings.Contains(err.Error(), tt.method) || !strings.Contains(err.Error(), "BAD_NAME") {
				t.Fatalf("error = %v, want method name and RPC fault", err)
			}
			if len(tt.args) > 0 && !strings.Contains(err.Error(), tt.args[0]) {
				t.Fatalf("error = %v, want process/group name %q", err, tt.args[0])
			}
		})
	}
}

func TestRPCClientBooleanFailures(t *testing.T) {
	for _, tt := range []struct {
		method string
		call   func(*RPCClient) error
		want   string
	}{
		{"supervisor.stopProcess", func(c *RPCClient) error { return c.StopProcess("job") }, "cannot stop process=job"},
		{"supervisor.startProcess", func(c *RPCClient) error { return c.StartProcess("job") }, "cannot start process=job"},
		{"supervisor.addProcessGroup", func(c *RPCClient) error { return c.AddProcessGroup("job") }, "add process group=job failed"},
		{"supervisor.removeProcessGroup", func(c *RPCClient) error { return c.RemoveProcessGroup("job") }, "remove process group=job failed"},
	} {
		t.Run(tt.method, func(t *testing.T) {
			client := newScriptedRPCClient(t, rpcTestStep{method: tt.method, args: []string{"job"}, response: rpcTestResponse(rpcTestFalse)})
			if err := tt.call(client); err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRPCClientResponseErrors(t *testing.T) {
	for _, tt := range []struct {
		name     string
		response string
		status   int
		want     string
	}{
		{"HTTP failure", "service unavailable", http.StatusServiceUnavailable, "503"},
		{"invalid XML", "<methodResponse>", http.StatusOK, "EOF"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newScriptedRPCClient(t, rpcTestStep{method: "supervisor.getAPIVersion", response: tt.response, status: tt.status})
			version, err := client.GetAPIVersion()
			if version != "" || err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "supervisor.getAPIVersion") {
				t.Fatalf("GetAPIVersion() = (%q, %v), want empty version and %q error", version, err, tt.want)
			}
		})
	}
}

func TestNewRPCClientInvalidURL(t *testing.T) {
	if client := NewRPCClient("://invalid"); client != nil {
		client.Close()
		t.Fatal("NewRPCClient returned a client for an invalid URL")
	}
}

func TestRPCClientRunningProcessMap(t *testing.T) {
	stopped := strings.Replace(rpcTestProcessValue, "<int>123</int>", "<int>0</int>", 1)
	second := strings.Replace(rpcTestProcessValue, "<int>123</int>", "<int>456</int>", 1)
	second = strings.Replace(second, "<string>worker</string>", "<string>api</string>", 1)
	for _, tt := range []struct {
		name     string
		response string
		want     map[int]ProcessInfo
		wantErr  bool
	}{
		{"mixed", rpcTestResponse(rpcTestArray(rpcTestProcessValue, stopped, second)), map[int]ProcessInfo{
			123: rpcTestProcessInfo(),
			456: func() ProcessInfo { p := rpcTestProcessInfo(); p.Pid = 456; p.Name = "api"; return p }(),
		}, false},
		{"empty", rpcTestResponse(rpcTestArray()), map[int]ProcessInfo{}, false},
		{"fault", rpcTestFault, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newScriptedRPCClient(t, rpcTestStep{method: "supervisor.getAllProcessInfo", response: tt.response})
			got, err := client.GetAllRunningProcessInfoMap()
			if (err != nil) != tt.wantErr || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("GetAllRunningProcessInfoMap() = (%#v, %v), want (%#v, error=%v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestRPCClientReloadConfigEmptyChanges(t *testing.T) {
	client := newScriptedRPCClient(t, rpcTestStep{method: "supervisor.reloadConfig", response: rpcTestReload(nil, nil, nil)})
	got, err := client.ReloadConfig()
	if err != nil || !reflect.DeepEqual(got, ReloadConfigResult{}) {
		t.Fatalf("ReloadConfig() = (%#v, %v), want empty changes", got, err)
	}
}

func TestRPCClientCallContext(t *testing.T) {
	for _, name := range []string{"cancel", "timeout"} {
		t.Run(name, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/xml")
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				close(started)
				<-release
				io.WriteString(w, rpcTestResponse(rpcTestString("3.0")))
			}))
			client := NewRPCClient(server.URL)
			// Release the blocked response before closing the client and server.
			t.Cleanup(func() { close(release); client.Close(); server.Close() })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				var reply VersionReply
				if name == "timeout" {
					result <- client.callTimeout("supervisor.getAPIVersion", &struct{}{}, &reply, 50*time.Millisecond)
				} else {
					result <- client.callContext(ctx, "supervisor.getAPIVersion", &struct{}{}, &reply)
				}
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not reach the server")
			}
			want := context.DeadlineExceeded
			if name == "cancel" {
				cancel()
				want = context.Canceled
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("call error = %v, want %v", err, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("call did not return after context completion")
			}
		})
	}
}

func rpcTestUpdateSteps() []rpcTestStep {
	return []rpcTestStep{
		{method: "supervisor.reloadConfig", response: rpcTestReload([]string{"new"}, []string{"changed"}, []string{"old"})},
		{method: "supervisor.getAllProcessInfo", response: rpcTestResponse(rpcTestArray(
			strings.Replace(rpcTestProcessValue, "<string>services</string>", "<string>old</string>", 1),
			strings.Replace(rpcTestProcessValue, "<string>services</string>", "<string>changed</string>", 1),
		))},
		{method: "supervisor.stopProcessGroup", args: []string{"old"}, response: rpcTestResponse(rpcTestArray(rpcTestStatusValue))},
		{method: "supervisor.removeProcessGroup", args: []string{"old"}, response: rpcTestResponse(rpcTestTrue)},
		{method: "supervisor.stopProcessGroup", args: []string{"changed"}, response: rpcTestResponse(rpcTestArray(rpcTestStatusValue))},
		{method: "supervisor.removeProcessGroup", args: []string{"changed"}, response: rpcTestResponse(rpcTestTrue)},
		{method: "supervisor.addProcessGroup", args: []string{"changed"}, response: rpcTestResponse(rpcTestTrue)},
		{method: "supervisor.addProcessGroup", args: []string{"new"}, response: rpcTestResponse(rpcTestTrue)},
	}
}

func TestRPCClientUpdateAllConfig(t *testing.T) {
	client := newScriptedRPCClient(t, rpcTestUpdateSteps()...)
	if err := client.UpdateAllConfig(); err != nil {
		t.Fatal(err)
	}
}

func TestRPCClientUpdateAllConfigStopsOnError(t *testing.T) {
	for index, step := range rpcTestUpdateSteps() {
		t.Run(fmt.Sprintf("step_%d_%s", index, step.method), func(t *testing.T) {
			steps := rpcTestUpdateSteps()[:index+1]
			steps[index].response = rpcTestFault
			client := newScriptedRPCClient(t, steps...)
			if err := client.UpdateAllConfig(); err == nil || !strings.Contains(err.Error(), step.method) || !strings.Contains(err.Error(), "BAD_NAME") {
				t.Fatalf("UpdateAllConfig() error = %v, want error from %s", err, step.method)
			}
		})
	}
}

func TestRPCClientUpdateAllConfigSkipsMissingGroups(t *testing.T) {
	for _, processes := range []string{
		rpcTestArray(),
		rpcTestArray(strings.Replace(rpcTestProcessValue, "<string>services</string>", "<string></string>", 1)),
	} {
		client := newScriptedRPCClient(t,
			rpcTestStep{method: "supervisor.reloadConfig", response: rpcTestReload([]string{"new"}, []string{"missing"}, []string{"missing"})},
			rpcTestStep{method: "supervisor.getAllProcessInfo", response: rpcTestResponse(processes)},
			rpcTestStep{method: "supervisor.addProcessGroup", args: []string{"new"}, response: rpcTestResponse(rpcTestTrue)},
		)
		if err := client.UpdateAllConfig(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRPCClientUpdateAllConfigSkipsFailedRemoval(t *testing.T) {
	// A per-process status 30 (FAILED) must leave the old group in place;
	// other additions should still be processed.
	failed := strings.Replace(rpcTestStatusValue, "<int>80</int>", "<int>30</int>", 1)
	client := newScriptedRPCClient(t,
		rpcTestStep{method: "supervisor.reloadConfig", response: rpcTestReload([]string{"new"}, nil, []string{"services"})},
		rpcTestStep{method: "supervisor.getAllProcessInfo", response: rpcTestResponse(rpcTestArray(rpcTestProcessValue))},
		rpcTestStep{method: "supervisor.stopProcessGroup", args: []string{"services"}, response: rpcTestResponse(rpcTestArray(rpcTestStatusValue, failed))},
		rpcTestStep{method: "supervisor.addProcessGroup", args: []string{"new"}, response: rpcTestResponse(rpcTestTrue)},
	)
	if err := client.UpdateAllConfig(); err != nil {
		t.Fatal(err)
	}
}

func TestRPCClientConcurrentReads(t *testing.T) {
	const count = 8
	steps := make([]rpcTestStep, count)
	for i := range steps {
		steps[i] = rpcTestStep{method: "supervisor.getAPIVersion", response: rpcTestResponse(rpcTestString("3.0"))}
	}
	client := newScriptedRPCClient(t, steps...)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Go(func() {
			if got, err := client.GetAPIVersion(); err != nil || got != "3.0" {
				t.Errorf("GetAPIVersion() = (%q, %v), want (3.0, nil)", got, err)
			}
		})
	}
	wg.Wait()
}

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
