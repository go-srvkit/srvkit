// Copyright © 2025 Johnnie Chen . All rights reserved.
// See accompanying LICENSE file

package supervisor

import (
	"context"
	"fmt"
	"net/rpc"
	"time"

	"alexejk.io/go-xmlrpc"
	"github.com/pkg/errors"
	"gopkg.in/srvkit.v1/logger"
)

const (
	ReadAPITimeout  = 30 * time.Second
	WriteAPITimeout = 60 * time.Second
)

// RPCClient the supervisor XML RPC client library
type RPCClient struct {
	*xmlrpc.Client
	serverUrl string
}

// NewRPCClient create a RPCClient object
func NewRPCClient(serverUrl string) *RPCClient {
	client, err := xmlrpc.NewClient(serverUrl)
	if err != nil {
		logger.Errorf("cannot create xmlrpc client: %v", err)
		return nil
	}
	return &RPCClient{
		serverUrl: serverUrl,
		Client:    client,
	}
}

func (r *RPCClient) callContext(ctx context.Context, serviceMethod string, args any, reply any) error {
	call := r.Go(serviceMethod, args, reply, make(chan *rpc.Call, 1))
	select {
	case done := <-call.Done:
		return done.Error
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *RPCClient) callTimeout(serviceMethod string, args any, reply any, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.callContext(ctx, serviceMethod, args, reply)
}

// GetAPIVersion Return the version of the RPC API used by supervisord
func (r *RPCClient) GetAPIVersion() (string, error) {
	var reply VersionReply
	var args = struct{}{}
	const method = "supervisor.getAPIVersion"
	if err := r.callTimeout(method, &args, &reply, ReadAPITimeout); err != nil {
		return "", errors.Wrap(err, method)
	}
	return reply.Value, nil
}

// GetSupervisordPID Return the PID of supervisord
func (r *RPCClient) GetSupervisordPID() (int, error) {
	var reply PIDReply
	var args = struct{}{}
	const method = "supervisor.getPID"
	if err := r.callTimeout(method, &args, &reply, ReadAPITimeout); err != nil {
		return 0, errors.Wrap(err, method)
	}
	return reply.Value, nil
}

// GetSupervisordState Return the PID of supervisord
func (r *RPCClient) GetSupervisordState() (ServerState, error) {
	var reply ServerStateReply
	var args = struct{}{}
	const method = "supervisor.getState"
	if err := r.callTimeout(method, &args, &reply, ReadAPITimeout); err != nil {
		return reply.Value, errors.Wrap(err, method)
	}
	return reply.Value, nil
}

// GetAllProcessInfo get all the processes of supervisor
func (r *RPCClient) GetAllProcessInfo() ([]ProcessInfo, error) {
	var reply AllProcessInfoReply
	var args = struct{}{}
	const method = "supervisor.getAllProcessInfo"
	if err := r.callTimeout(method, &args, &reply, ReadAPITimeout); err != nil {
		return reply.Value, errors.Wrap(err, method)
	}
	return reply.Value, nil
}

// GetAllRunningProcessInfoMap get all running processes info map
func (r *RPCClient) GetAllRunningProcessInfoMap() (map[int]ProcessInfo, error) {
	processList, err := r.GetAllProcessInfo()
	if err != nil {
		return nil, err
	}
	var infoMap = map[int]ProcessInfo{}
	for _, proc := range processList {
		if proc.Pid > 0 {
			infoMap[proc.Pid] = proc
		}
	}
	return infoMap, nil
}

// GetProcessInfo get the process information of one program
func (r *RPCClient) GetProcessInfo(name string) (ProcessInfo, error) {
	var reply ProcessInfoReply
	var args = struct{ Name string }{name}
	const method = "supervisor.getProcessInfo"
	if err := r.callTimeout(method, &args, &reply, ReadAPITimeout); err != nil {
		return reply.Value, errors.Wrap(err, method+":"+name)
	}
	return reply.Value, nil
}

// StopProcess Stop a process named by name
func (r *RPCClient) StopProcess(name string) error {
	var reply BooleanReply
	var args = struct{ Name string }{name}
	const method = "supervisor.stopProcess"
	if err := r.callTimeout(method, &args, &reply, WriteAPITimeout); err != nil {
		return errors.Wrap(err, method+":"+name)
	}
	if !reply.Value {
		return fmt.Errorf("cannot stop process=%s", name)
	}
	return nil
}

// StartProcess Start a process
func (r *RPCClient) StartProcess(name string) error {
	var reply BooleanReply
	var args = struct{ Name string }{name}
	const method = "supervisor.startProcess"
	if err := r.callTimeout(method, &args, &reply, WriteAPITimeout); err != nil {
		return errors.Wrap(err, method+":"+name)
	}
	if !reply.Value {
		return fmt.Errorf("cannot start process=%s", name)
	}
	return nil
}

// StopProcessGroup Stop all processes in the process group named ‘name’
func (r *RPCClient) StopProcessGroup(name string) ([]ProcessStatusInfo, error) {
	var reply StartStopManyReply
	var args = struct{ Name string }{name}
	const method = "supervisor.stopProcessGroup"
	if err := r.callTimeout(method, &args, &reply, WriteAPITimeout); err != nil {
		return nil, errors.Wrap(err, method+":"+name)
	}
	return reply.Value, nil
}

// StartProcessGroup Start all processes in the group named ‘name’
func (r *RPCClient) StartProcessGroup(name string) ([]ProcessStatusInfo, error) {
	var reply StartStopManyReply
	var args = struct{ Name string }{name}
	const method = "supervisor.startProcessGroup"
	if err := r.callTimeout(method, &args, &reply, WriteAPITimeout); err != nil {
		return nil, errors.Wrap(err, method+":"+name)
	}
	return reply.Value, nil
}

// stopAllProcesses Stop all processes in the process list
func (r *RPCClient) stopAllProcesses() ([]ProcessStatusInfo, error) {
	var reply StartStopManyReply
	var args = struct{}{}
	const method = "supervisor.stopAllProcesses"
	if err := r.callTimeout(method, &args, &reply, 2*WriteAPITimeout); err != nil {
		return nil, errors.Wrap(err, method)
	}
	return reply.Value, nil
}

// StartAllProcesses Start all processes listed in the configuration file
func (r *RPCClient) StartAllProcesses() ([]ProcessStatusInfo, error) {
	var reply StartStopManyReply
	var args = struct{}{}
	const method = "supervisor.startAllProcesses"
	if err := r.callTimeout(method, &args, &reply, 2*WriteAPITimeout); err != nil {
		return nil, errors.Wrap(err, method)
	}
	return reply.Value, nil
}

// ReloadConfig Reload the configuration
func (r *RPCClient) ReloadConfig() (ReloadConfigResult, error) {
	var result ReloadConfigResult
	var reply ReloadConfigReply
	var args = struct{}{}
	const method = "supervisor.reloadConfig"
	if err := r.callTimeout(method, &args, &reply, ReadAPITimeout); err != nil {
		return result, errors.Wrap(err, method)
	}
	if len(reply.Value) > 0 && len(reply.Value[0]) >= 3 {
		var changeSet = reply.Value[0]
		result.Added = changeSet[0]
		result.Changed = changeSet[1]
		result.Removed = changeSet[2]
	}
	return result, nil
}

// AddProcessGroup Update the config for a running process from config file.
func (r *RPCClient) AddProcessGroup(name string) error {
	var reply BooleanReply
	var args = struct{ Name string }{name}
	const method = "supervisor.addProcessGroup"
	if err := r.callTimeout(method, &args, &reply, WriteAPITimeout); err != nil {
		return errors.Wrap(err, method+":"+name)
	}
	if !reply.Value {
		return fmt.Errorf("add process group=%s failed", name)
	}
	return nil
}

// RemoveProcessGroup Remove a stopped process from the active configuration.
func (r *RPCClient) RemoveProcessGroup(name string) error {
	var reply BooleanReply
	var args = struct{ Name string }{name}
	const method = "supervisor.removeProcessGroup"
	if err := r.callTimeout(method, &args, &reply, WriteAPITimeout); err != nil {
		return errors.Wrap(err, method+":"+name)
	}
	if !reply.Value {
		return fmt.Errorf("remove process group=%s failed", name)
	}
	return nil
}

// UpdateAllConfig 更新所有配置
// see supervisord do_update() code below:
// https://github.com/Supervisor/supervisor/blob/4.2.5/supervisor/supervisorctl.py
func (r *RPCClient) UpdateAllConfig() error {
	result, err := r.ReloadConfig()
	if err != nil {
		return err
	}
	var validGroupNames = map[string]bool{}
	allProcs, err := r.GetAllProcessInfo()
	if err != nil {
		return err
	}
	for _, proc := range allProcs {
		if proc.Group != "" {
			validGroupNames[proc.Group] = true
		}
	}
	for _, gname := range result.Removed {
		if _, found := validGroupNames[gname]; !found {
			continue
		}
		stopped, err := r.StopProcessGroup(gname)
		if err != nil {
			return err
		}
		logger.Infof("stopped process group=%s", gname)
		var hasFailed = false
		for _, proc := range stopped {
			if proc.Status == 30 {
				logger.Errorf("%s has problems; not removing", proc.Name)
				hasFailed = true
			}
		}
		if hasFailed {
			continue
		}
		if err = r.RemoveProcessGroup(gname); err != nil {
			return err
		}
	}
	for _, gname := range result.Changed {
		if _, found := validGroupNames[gname]; !found {
			continue
		}
		if _, err = r.StopProcessGroup(gname); err != nil {
			return err
		}
		logger.Infof("stopped process group=%s", gname)
		if err = r.RemoveProcessGroup(gname); err != nil {
			return err
		}
		if err = r.AddProcessGroup(gname); err != nil {
			return err
		}
		logger.Infof("updated process group=%s", gname)
	}
	for _, gname := range result.Added {
		if err = r.AddProcessGroup(gname); err != nil {
			return err
		}
		logger.Infof("added process group=%s", gname)
	}
	return nil
}
