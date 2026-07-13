// Copyright © 2025 Johnnie Chen . All rights reserved.
// See accompanying LICENSE file

package supervisor

// ProcessInfo the running process information
type ProcessInfo struct {
	Name          string `xml:"name" json:"name"`
	Group         string `xml:"group" json:"group"`
	Description   string `xml:"description" json:"description"`
	Pid           int    `xml:"pid" json:"pid"`
	Start         int    `xml:"start" json:"start"`
	Stop          int    `xml:"stop" json:"stop"`
	Now           int    `xml:"now" json:"now"`
	State         int    `xml:"state" json:"state"`
	Statename     string `xml:"statename" json:"statename"`
	Spawnerr      string `xml:"spawnerr" json:"spawnerr"`
	Exitstatus    int    `xml:"exitstatus" json:"exitstatus"`
	Logfile       string `xml:"logfile" json:"logfile"`
	StdoutLogfile string `xml:"stdout_logfile" json:"stdout_logfile"`
	StderrLogfile string `xml:"stderr_logfile" json:"stderr_logfile"`
}

type ProcessStatusInfo struct {
	Name        string `xml:"name" json:"name"`
	Group       string `xml:"group" json:"group"`
	Description string `xml:"description" json:"description"`
	Status      int    `xml:"pid" json:"pid"`
}

type VersionReply struct {
	Value string
}

type PIDReply struct {
	Value int
}

type ServerState struct {
	Statecode int    `xml:"statecode" json:"statecode"`
	Statename string `xml:"statename" json:"statename"`
}

type ServerStateReply struct {
	Value ServerState
}

type AllProcessInfoReply struct {
	Value []ProcessInfo
}

type ProcessInfoReply struct {
	Value ProcessInfo
}

type BooleanReply struct {
	Value bool
}

type StartStopManyReply struct {
	Value []ProcessStatusInfo
}

type ReloadConfigResult struct {
	Added   []string
	Changed []string
	Removed []string
}

// ReloadConfigReply 重读配置，python格式为[[[], [], []]]
type ReloadConfigReply struct {
	Value [][][]string
}
