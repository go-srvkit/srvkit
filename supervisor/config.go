// Copyright © 2025 Johnnie Chen . All rights reserved.
// See accompanying LICENSE file

package supervisor

import (
	"io"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
	"server/base/misc"
)

func init() {
	ini.PrettyFormat = false
	ini.PrettyEqual = false
}

type INIConfig struct {
	Programs []ProgramConfig `json:"programs"`
	Groups   []ProgramGroup  `json:"groups"`
	Sections []SimpleSection `json:"sections,omitempty"`
}

type SimpleSection struct {
	Name      string            `json:"name"`
	KeyValues map[string]string `json:"key_values"`
}

type ProgramGroup struct {
	Name     string   `json:"name"`
	Programs []string `json:"programs"`
}

type ProgramConfig struct {
	SectionName           string `json:"name" ini:"-"`
	Command               string `json:"command" ini:"command"`
	ProcessName           string `json:"process_name,omitempty" ini:"process_name"`
	NumProcs              int    `json:"numprocs,omitempty" ini:"numprocs,omitempty"`
	Directory             string `json:"directory" ini:"directory"`
	Umask                 string `json:"umask,omitempty" ini:"umask,omitempty"`
	Priority              int    `json:"priority,omitempty" ini:"priority,omitempty"`
	AutoStart             bool   `json:"autostart,omitempty" ini:"autostart,omitempty"`
	StartSecs             int    `json:"startsecs,omitempty" ini:"startsecs,omitempty"`
	StartRetries          int    `json:"startretries,omitempty" ini:"startretries,omitempty"`
	AutoRestart           string `json:"autorestart,omitempty" ini:"auto_restart"`
	ExitCodes             []int  `json:"exitcodes,omitempty" ini:"exitcodes,omitempty"`
	StopSignal            string `json:"stopsignal,omitempty" ini:"stopsignal,omitempty"`
	StopWaitSecs          int    `json:"stopwaitsecs,omitempty" ini:"stopwaitsecs,omitempty"`
	StopAsGroup           bool   `json:"stopasgroup,omitempty" ini:"stopasgroup,omitempty"`
	KillAsGroup           bool   `json:"killasgroup,omitempty" ini:"killasgroup,omitempty"`
	User                  string `json:"user,omitempty" ini:"user,omitempty"`
	RedirectStderr        bool   `json:"redirect_stderr,omitempty" ini:"redirect_stderr,omitempty"`
	StdoutLogfile         string `json:"stdout_logfile,omitempty" ini:"stdout_logfile,omitempty"`
	StdoutLogfileMaxBytes string `json:"stdout_logfile_maxbytes,omitempty" ini:"stdout_logfile_maxbytes,omitempty"`
	StdoutLogfileBackups  int    `json:"stdout_logfile_backups,omitempty" ini:"stdout_logfile_backups,omitempty"`
	StdoutCaptureMaxBytes string `json:"stdout_capture_maxbytes,omitempty" ini:"stdout_capture_maxbytes,omitempty"`
	StdoutEventsEnabled   bool   `json:"stdout_events_enabled,omitempty" ini:"stdout_events_enabled,omitempty"`
	StdoutSyslog          bool   `json:"stdout_syslog,omitempty" ini:"stdout_syslog,omitempty"`
	StderrLogfile         string `json:"stderr_logfile,omitempty" ini:"stderr_logfile,omitempty"`
	StderrLogfileMaxBytes string `json:"stderr_logfile_maxbytes,omitempty" ini:"stderr_logfile_maxbytes,omitempty"`
	StderrLogfileBackups  int    `json:"stderr_logfile_backups,omitempty" ini:"stderr_logfile_backups,omitempty"`
	StderrCaptureMaxBytes string `json:"stderr_capture_maxbytes,omitempty" ini:"stderr_capture_maxbytes,omitempty"`
	StderrEventsEnabled   bool   `json:"stderr_events_enabled,omitempty" ini:"stderr_events_enabled,omitempty"`
	StderrSyslog          bool   `json:"stderr_syslog,omitempty" ini:"stderr_syslog,omitempty"`
	Environment           string `json:"environment" ini:"environment"`
	ServerUrl             string `json:"serverurl,omitempty" ini:"serverurl,omitempty"`
}

func addProgramToSection(prog ProgramConfig, section *ini.Section) {
	section.NewKey("command", prog.Command)
	if prog.ProcessName != "" {
		section.NewKey("process_name", prog.ProcessName)
	}
	if prog.NumProcs > 0 {
		section.NewKey("numprocs", strconv.Itoa(prog.NumProcs))
	}
	if prog.Umask != "" {
		section.NewKey("umask", prog.Umask)
	}
	if prog.Priority > 0 {
		section.NewKey("priority", strconv.Itoa(prog.Priority))
	}
	section.NewKey("autostart", strconv.FormatBool(prog.AutoStart))
	if prog.StartSecs > 0 {
		section.NewKey("startsecs", strconv.Itoa(prog.StartSecs))
	}
	if prog.StartRetries > 0 {
		section.NewKey("startretries", strconv.Itoa(prog.StartRetries))
	}
	if prog.AutoRestart != "" {
		section.NewKey("autorestart", prog.AutoRestart)
	}
	section.NewKey("directory", prog.Directory)
	if len(prog.ExitCodes) > 0 {
		var sb strings.Builder
		for i, code := range prog.ExitCodes {
			sb.WriteString(strconv.Itoa(code))
			if i+1 < len(prog.ExitCodes) {
				sb.WriteString(",")
			}
		}
		section.NewKey("exitcodes", sb.String())
	}
	if prog.StopSignal != "" {
		section.NewKey("stopsignal", prog.StopSignal)
	}
	if prog.StopWaitSecs > 0 {
		section.NewKey("stopwaitsecs", strconv.Itoa(prog.StopWaitSecs))
	}
	if prog.StopAsGroup {
		section.NewKey("stopasgroup", strconv.FormatBool(prog.StopAsGroup))
	}
	if prog.KillAsGroup {
		section.NewKey("killasgroup", strconv.FormatBool(prog.KillAsGroup))
	}
	if prog.User != "" {
		section.NewKey("user", prog.User)
	}
	if prog.RedirectStderr {
		section.NewKey("redirect_stderr", strconv.FormatBool(prog.RedirectStderr))
	}
	section.NewKey("redirect_stderr", strconv.FormatBool(prog.RedirectStderr))

	if prog.StdoutLogfile != "" {
		section.NewKey("stdout_logfile", prog.StdoutLogfile)
	}
	if prog.StdoutLogfileMaxBytes != "" {
		section.NewKey("stdout_logfile_maxbytes", prog.StdoutLogfileMaxBytes)
	}
	if prog.StdoutLogfileBackups > 0 {
		section.NewKey("stdout_logfile_backups", strconv.Itoa(prog.StdoutLogfileBackups))
	}
	if prog.StdoutCaptureMaxBytes != "" {
		section.NewKey("stdout_capture_maxbytes", prog.StdoutCaptureMaxBytes)
	}
	if prog.StdoutEventsEnabled {
		section.NewKey("stdout_events_enabled", strconv.FormatBool(prog.StdoutEventsEnabled))
	}
	if prog.StdoutSyslog {
		section.NewKey("stdout_syslog", strconv.FormatBool(prog.StdoutSyslog))
	}
	if prog.StderrLogfile != "" {
		section.NewKey("stderr_logfile", prog.StderrLogfile)
	}
	if prog.StderrLogfileMaxBytes != "" {
		section.NewKey("stderr_logfile_maxbytes", prog.StderrLogfileMaxBytes)
	}
	if prog.StderrLogfileBackups > 0 {
		section.NewKey("stderr_logfile_backups", strconv.Itoa(prog.StderrLogfileBackups))
	}
	if prog.StderrCaptureMaxBytes != "" {
		section.NewKey("stderr_capture_maxbytes", prog.StderrCaptureMaxBytes)
	}
	if prog.StderrEventsEnabled {
		section.NewKey("stderr_events_enabled", strconv.FormatBool(prog.StderrEventsEnabled))
	}
	if prog.StderrSyslog {
		section.NewKey("stderr_syslog", strconv.FormatBool(prog.StderrSyslog))
	}
	section.NewKey("environment", prog.Environment)
	if prog.ServerUrl != "" {
		section.NewKey("serverurl", prog.ServerUrl)
	}
}

func ReadSupervisorConfig(data []byte) (*INIConfig, error) {
	file, err := ini.Load(data)
	if err != nil {
		return nil, err
	}
	var conf = new(INIConfig)
	var sections = file.Sections()
	for _, section := range sections {
		var secName = section.Name()
		if secName == ini.DefaultSection {
			continue
		}
		if strings.HasPrefix(secName, "program:") {
			var program = ProgramConfig{SectionName: secName[8:]}
			if err = section.MapTo(&program); err != nil {
				return nil, err
			}
			conf.Programs = append(conf.Programs, program)
		} else if strings.HasPrefix(secName, "group:") {
			var group = ProgramGroup{Name: secName[6:]}
			key, err := section.GetKey("programs")
			if err != nil {
				return nil, err
			}
			group.Programs = strings.Split(key.Value(), ",")
			conf.Groups = append(conf.Groups, group)
		} else {
			var item = SimpleSection{Name: secName, KeyValues: section.KeysHash()}
			conf.Sections = append(conf.Sections, item)
		}
	}
	return conf, nil
}

func WriteSupervisorConfig(conf *INIConfig, w io.Writer) error {
	var file = ini.Empty()
	for _, program := range conf.Programs {
		section, _ := file.NewSection("program:" + program.SectionName)
		addProgramToSection(program, section)
	}
	for _, group := range conf.Groups {
		section, _ := file.NewSection("group:" + group.Name)
		section.NewKey("programs", strings.Join(group.Programs, ","))
	}
	for _, item := range conf.Sections {
		if item.Name == "" {
			continue
		}
		section, _ := file.NewSection(item.Name)
		var keys = misc.MapOrderedKeys(item.KeyValues)
		for i := 0; i < len(keys); i++ {
			var key = keys[i]
			if key != "" {
				section.NewKey(key, item.KeyValues[key])
			}
		}
	}
	_, err := file.WriteTo(w)
	return err
}
