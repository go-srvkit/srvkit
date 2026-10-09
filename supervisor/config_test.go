// Copyright © 2025 Johnnie Chen . All rights reserved.
// See accompanying LICENSE file

package supervisor

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
)

const fullConfigINI = `[program:worker]
command=/opt/app/worker --label "hello world" --id=%(process_num)s
process_name=%(program_name)s_%(process_num)02d
numprocs=2
umask=027
priority=100
autostart=true
startsecs=2
startretries=4
autorestart=unexpected
directory=/opt/app
exitcodes=0,2,10
stopsignal=QUIT
stopwaitsecs=20
stopasgroup=true
killasgroup=true
user=app
redirect_stderr=true
stdout_logfile=/tmp/worker_%(process_num)s.log
stdout_logfile_maxbytes=10MB
stdout_logfile_backups=3
stdout_capture_maxbytes=1KB
stdout_events_enabled=true
stdout_syslog=true
stderr_logfile=/tmp/worker_%(process_num)s.err
stderr_logfile_maxbytes=5MB
stderr_logfile_backups=2
stderr_capture_maxbytes=2KB
stderr_events_enabled=true
stderr_syslog=true
environment=MODE="production",LABEL="hello, world",URI="/first%%20name",OPTION="a=b"
serverurl=AUTO

[program:api]
command=/opt/app/api
autostart=false
autorestart=false
redirect_stderr=false
environment=

[group:services]
programs=worker,api

[rpcinterface:supervisor]
supervisor.rpcinterface_factory=supervisor.rpcinterface:make_main_rpcinterface

[supervisorctl]
password=secret
serverurl=unix:///tmp/supervisor.sock
username=admin
`

func fullConfigFixture() *INIConfig {
	return &INIConfig{
		Programs: []ProgramConfig{
			{
				SectionName: "worker",
				Command:     `/opt/app/worker --label "hello world" --id=%(process_num)s`,
				ProcessName: "%(program_name)s_%(process_num)02d",
				NumProcs:    2, Directory: "/opt/app", Umask: "027", Priority: 100,
				AutoStart: true, StartSecs: 2, StartRetries: 4, AutoRestart: "unexpected",
				ExitCodes: []int{0, 2, 10}, StopSignal: "QUIT", StopWaitSecs: 20,
				StopAsGroup: true, KillAsGroup: true, User: "app", RedirectStderr: true,
				StdoutLogfile: "/tmp/worker_%(process_num)s.log", StdoutLogfileMaxBytes: "10MB",
				StdoutLogfileBackups: 3, StdoutCaptureMaxBytes: "1KB",
				StdoutEventsEnabled: true, StdoutSyslog: true,
				StderrLogfile: "/tmp/worker_%(process_num)s.err", StderrLogfileMaxBytes: "5MB",
				StderrLogfileBackups: 2, StderrCaptureMaxBytes: "2KB",
				StderrEventsEnabled: true, StderrSyslog: true,
				Environment: `MODE="production",LABEL="hello, world",URI="/first%%20name",OPTION="a=b"`,
				ServerUrl:   "AUTO",
			},
			{SectionName: "api", Command: "/opt/app/api", AutoRestart: "false"},
		},
		Groups: []ProgramGroup{{Name: "services", Programs: []string{"worker", "api"}}},
		Sections: []SimpleSection{
			{Name: "rpcinterface:supervisor", KeyValues: map[string]string{
				"supervisor.rpcinterface_factory": "supervisor.rpcinterface:make_main_rpcinterface",
			}},
			{Name: "supervisorctl", KeyValues: map[string]string{
				"serverurl": "unix:///tmp/supervisor.sock", "username": "admin", "password": "secret",
			}},
		},
	}
}

func TestReadSupervisorConfig(t *testing.T) {
	conf, err := ReadSupervisorConfig([]byte("# supervisor configuration\n" + fullConfigINI))
	if err != nil {
		t.Fatal(err)
	}
	if want := fullConfigFixture(); !reflect.DeepEqual(conf, want) {
		t.Fatalf("parsed config = %#v, want %#v", conf, want)
	}
}

func TestWriteSupervisorConfig(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSupervisorConfig(fullConfigFixture(), &buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != fullConfigINI {
		t.Fatalf("generated config:\n%s\nwant:\n%s", got, fullConfigINI)
	}
}

func TestSupervisorConfigRoundTrip(t *testing.T) {
	want := fullConfigFixture()
	var buf bytes.Buffer
	if err := WriteSupervisorConfig(want, &buf); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSupervisorConfig(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip config = %#v, want %#v", got, want)
	}
}

func TestWriteSupervisorConfigDefaults(t *testing.T) {
	conf := &INIConfig{Programs: []ProgramConfig{{SectionName: "job", Command: "/bin/true"}}}
	var buf bytes.Buffer
	if err := WriteSupervisorConfig(conf, &buf); err != nil {
		t.Fatal(err)
	}
	// The generator explicitly disables autostart and autorestart by default.
	// An unspecified directory must be omitted, since directory= prevents startup.
	const want = "[program:job]\ncommand=/bin/true\nautostart=false\nautorestart=false\nredirect_stderr=false\nenvironment=\n"
	if got := buf.String(); got != want {
		t.Fatalf("generated config = %q, want %q", got, want)
	}
	if conf.Programs[0].AutoRestart != "" {
		t.Fatal("writing config mutated the caller's restart policy")
	}
}

func TestSupervisorConfigAutoRestart(t *testing.T) {
	for _, policy := range []string{"false", "true", "unexpected"} {
		t.Run(policy, func(t *testing.T) {
			input := "[program:job]\ncommand=/bin/true\nautorestart=" + policy + "\n"
			conf, err := ReadSupervisorConfig([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if got := conf.Programs[0].AutoRestart; got != policy {
				t.Fatalf("autorestart = %q, want %q", got, policy)
			}
			var buf bytes.Buffer
			if err := WriteSupervisorConfig(conf, &buf); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(buf.Bytes(), []byte("\nautorestart="+policy+"\n")) {
				t.Fatalf("restart policy missing from generated config: %s", buf.String())
			}
		})
	}
}

func TestReadSupervisorConfigZeroValues(t *testing.T) {
	const input = `[program:job]
command=/bin/true
priority=0
autostart=false
startsecs=0
startretries=0
stopwaitsecs=0
stdout_logfile_backups=0
stderr_logfile_backups=0
exitcodes=0
`
	conf, err := ReadSupervisorConfig([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []ProgramConfig{{SectionName: "job", Command: "/bin/true", ExitCodes: []int{0}}}
	if !reflect.DeepEqual(conf.Programs, want) {
		t.Fatalf("parsed programs = %#v, want %#v", conf.Programs, want)
	}
}

func TestReadSupervisorConfigEmpty(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{name: "nil"},
		{name: "empty", data: []byte{}},
		{name: "comments", data: []byte("# comment\n; another comment\n\n")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			conf, err := ReadSupervisorConfig(tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(conf, &INIConfig{}) {
				t.Fatalf("parsed config = %#v, want empty config", conf)
			}
		})
	}
}

func TestReadSupervisorConfigErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
	}{
		{name: "unclosed section", data: "[program:job\ncommand=/bin/true\n"},
		{name: "missing delimiter", data: "[program:job]\ncommand\n"},
		{name: "group without programs", data: "[group:services]\npriority=10\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			conf, err := ReadSupervisorConfig([]byte(tt.data))
			if err == nil || conf != nil {
				t.Fatalf("ReadSupervisorConfig() = (%#v, %v), want (nil, error)", conf, err)
			}
		})
	}
}

func TestWriteSupervisorConfigSimpleSections(t *testing.T) {
	conf := &INIConfig{Sections: []SimpleSection{
		{Name: "", KeyValues: map[string]string{"ignored": "value"}},
		{Name: "supervisord", KeyValues: map[string]string{
			"": "ignored", "pidfile": "/tmp/supervisor.pid", "logfile": "/tmp/supervisor.log",
		}},
	}}
	var buf bytes.Buffer
	if err := WriteSupervisorConfig(conf, &buf); err != nil {
		t.Fatal(err)
	}
	const want = "[supervisord]\nlogfile=/tmp/supervisor.log\npidfile=/tmp/supervisor.pid\n"
	if got := buf.String(); got != want {
		t.Fatalf("generated config = %q, want %q", got, want)
	}
}

func TestWriteSupervisorConfigEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSupervisorConfig(&INIConfig{}, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty config generated %q", buf.String())
	}
}

type configErrorWriter struct{ err error }

func (w configErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteSupervisorConfigWriterErrors(t *testing.T) {
	sentinel := errors.New("writer failed")
	for _, tt := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{name: "writer error", writer: configErrorWriter{err: sentinel}, want: sentinel},
		{name: "short write", writer: configErrorWriter{}, want: io.ErrShortWrite},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteSupervisorConfig(fullConfigFixture(), tt.writer)
			if !errors.Is(err, tt.want) {
				t.Fatalf("WriteSupervisorConfig() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestReadSupervisorConfigFile(t *testing.T) {
	path := os.Getenv("TEST_INI_FILE")
	if path == "" {
		t.Skip("set TEST_INI_FILE to test a local supervisor config")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	conf, err := ReadSupervisorConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteSupervisorConfig(conf, &buf); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSupervisorConfig(buf.Bytes()); err != nil {
		t.Fatalf("generated config cannot be read: %v", err)
	}
}
