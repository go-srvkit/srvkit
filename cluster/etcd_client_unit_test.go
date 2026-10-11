// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type stubKV struct {
	clientv3.KV
	get func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	put func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error)
	del func(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error)
	txn func(context.Context) clientv3.Txn
}

func (s *stubKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if s.get != nil {
		return s.get(ctx, key, opts...)
	}
	return &clientv3.GetResponse{Header: &pb.ResponseHeader{Revision: 100}}, ctx.Err()
}
func (s *stubKV) Put(ctx context.Context, key, value string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	if s.put != nil {
		return s.put(ctx, key, value, opts...)
	}
	return &clientv3.PutResponse{Header: &pb.ResponseHeader{Revision: 101}}, ctx.Err()
}
func (s *stubKV) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	if s.del != nil {
		return s.del(ctx, key, opts...)
	}
	return &clientv3.DeleteResponse{Deleted: 1}, ctx.Err()
}
func (s *stubKV) Txn(ctx context.Context) clientv3.Txn {
	if s.txn != nil {
		return s.txn(ctx)
	}
	return &stubTxn{}
}

type stubTxn struct {
	clientv3.Txn
	cmps   []clientv3.Cmp
	ops    []clientv3.Op
	commit func() (*clientv3.TxnResponse, error)
}

func (s *stubTxn) If(cmps ...clientv3.Cmp) clientv3.Txn { s.cmps = cmps; return s }
func (s *stubTxn) Then(ops ...clientv3.Op) clientv3.Txn { s.ops = ops; return s }
func (s *stubTxn) Commit() (*clientv3.TxnResponse, error) {
	if s.commit != nil {
		return s.commit()
	}
	return &clientv3.TxnResponse{Succeeded: true}, nil
}

type stubLease struct {
	clientv3.Lease
	grant  func(context.Context, int64) (*clientv3.LeaseGrantResponse, error)
	revoke func(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)
	keep   func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	ttl    func(context.Context, clientv3.LeaseID) (*clientv3.LeaseTimeToLiveResponse, error)
}

func (s *stubLease) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	if s.grant != nil {
		return s.grant(ctx, ttl)
	}
	return &clientv3.LeaseGrantResponse{ID: 7, TTL: ttl}, ctx.Err()
}
func (s *stubLease) Revoke(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	if s.revoke != nil {
		return s.revoke(ctx, id)
	}
	return &clientv3.LeaseRevokeResponse{}, ctx.Err()
}
func (s *stubLease) KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if s.keep != nil {
		return s.keep(ctx, id)
	}
	return make(chan *clientv3.LeaseKeepAliveResponse), ctx.Err()
}
func (s *stubLease) TimeToLive(ctx context.Context, id clientv3.LeaseID, _ ...clientv3.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error) {
	if s.ttl != nil {
		return s.ttl(ctx, id)
	}
	return &clientv3.LeaseTimeToLiveResponse{TTL: 5}, ctx.Err()
}
func (s *stubLease) Close() error { return nil }

type stubWatcher struct {
	clientv3.Watcher
	watch func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan
}

func (s *stubWatcher) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return s.watch(ctx, key, opts...)
}
func (s *stubWatcher) Close() error { return nil }

type stubMaintenance struct {
	clientv3.Maintenance
	status func(context.Context, string) (*clientv3.StatusResponse, error)
}

func (s *stubMaintenance) Status(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error) {
	return s.status(ctx, endpoint)
}

func fakeEtcdClient(t *testing.T) (*EtcdClient, *clientv3.Client, *stubKV, *stubLease) {
	t.Helper()
	cli := clientv3.NewCtxClient(context.Background())
	kv, lease := &stubKV{}, &stubLease{}
	cli.KV, cli.Lease = kv, lease
	c := NewEtcdClient("fake:2379", "/ROOT", "", "")
	c.SetVerbose(0)
	c.client = cli
	t.Cleanup(c.Close)
	return c, cli, kv, lease
}

func unitContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func receive[T any](t *testing.T, ctx context.Context, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-ctx.Done():
		t.Fatalf("waiting for test signal: %v", ctx.Err())
	}
	var zero T
	return zero
}

func TestEtcdClient_UnitInit(t *testing.T) {
	t.Run("empty endpoints", func(t *testing.T) {
		c := NewEtcdClient(" , , ", "/ROOT", "", "")
		require.ErrorIs(t, c.Init(t.Context()), ErrEmptyEndpoint)
		c.Close()
		require.ErrorIs(t, c.Init(t.Context()), ErrClientClosed)
	})
	t.Run("fallback and repeated init", func(t *testing.T) {
		c := NewEtcdClient(" first:2379, second:2379, ", "/ROOT", "user", "pass")
		t.Cleanup(c.Close)
		cli := clientv3.NewCtxClient(t.Context())
		var checked []string
		var checkedMu sync.Mutex
		firstChecked := make(chan struct{})
		cli.Maintenance = &stubMaintenance{status: func(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error) {
			_, bounded := ctx.Deadline()
			assert.True(t, bounded)
			checkedMu.Lock()
			checked = append(checked, endpoint)
			checkedMu.Unlock()
			if endpoint == "first:2379" {
				close(firstChecked)
				return nil, errors.New("unreachable")
			}
			select {
			case <-firstChecked:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return &clientv3.StatusResponse{}, nil
		}}
		var created int
		factory := func(config clientv3.Config) (*clientv3.Client, error) {
			created++
			assert.Equal(t, []string{"first:2379", "second:2379"}, config.Endpoints)
			assert.Equal(t, "user", config.Username)
			assert.Equal(t, "pass", config.Password)
			return cli, nil
		}
		require.NoError(t, c.initClient(t.Context(), factory))
		require.NoError(t, c.initClient(t.Context(), factory))
		require.Equal(t, 1, created)
		require.ElementsMatch(t, []string{"first:2379", "second:2379"}, checked)
		require.NoError(t, cli.Ctx().Err())
		c.Close()
		require.ErrorIs(t, cli.Ctx().Err(), context.Canceled)
	})
	t.Run("status failure closes unpublished client", func(t *testing.T) {
		c := NewEtcdClient("fake", "/ROOT", "", "")
		cli := clientv3.NewCtxClient(t.Context())
		failure := errors.New("status failed")
		cli.Maintenance = &stubMaintenance{status: func(context.Context, string) (*clientv3.StatusResponse, error) { return nil, failure }}
		require.ErrorIs(t, c.initClient(t.Context(), func(clientv3.Config) (*clientv3.Client, error) { return cli, nil }), failure)
		require.ErrorIs(t, cli.Ctx().Err(), context.Canceled)
		_, err := c.getClient()
		require.ErrorIs(t, err, ErrClientNotInitialized)
	})
	t.Run("factory error", func(t *testing.T) {
		c := NewEtcdClient("fake", "/ROOT", "", "")
		failure := errors.New("dial failed")
		require.ErrorIs(t, c.initClient(t.Context(), func(clientv3.Config) (*clientv3.Client, error) { return nil, failure }), failure)
	})
	t.Run("cancelled parent", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		c := NewEtcdClient("fake", "/ROOT", "", "")
		require.ErrorIs(t, c.Init(ctx), context.Canceled)
	})
}

func TestEtcdClient_UnitInitUnresponsiveEndpoint(t *testing.T) {
	ctx := unitContext(t)
	c := NewEtcdClient("blocked,healthy", "/ROOT", "", "")
	t.Cleanup(c.Close)
	cli := clientv3.NewCtxClient(ctx)
	blocked, exited := make(chan struct{}), make(chan struct{})
	cli.Maintenance = &stubMaintenance{status: func(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error) {
		if endpoint == "blocked" {
			close(blocked)
			<-ctx.Done()
			close(exited)
			return nil, ctx.Err()
		}
		select {
		case <-blocked:
			return &clientv3.StatusResponse{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	require.NoError(t, c.initClient(ctx, func(clientv3.Config) (*clientv3.Client, error) { return cli, nil }))
	receive(t, ctx, exited)
	require.NoError(t, ctx.Err(), "healthy member must succeed before the initialization timeout")
}

func TestEtcdClient_UnitConcurrentInit(t *testing.T) {
	ctx := unitContext(t)
	c := NewEtcdClient("fake", "/ROOT", "", "")
	t.Cleanup(c.Close)
	entered, release := make(chan struct{}, 2), make(chan struct{})
	clients := []*clientv3.Client{clientv3.NewCtxClient(ctx), clientv3.NewCtxClient(ctx)}
	for _, cli := range clients {
		cli.Maintenance = &stubMaintenance{status: func(context.Context, string) (*clientv3.StatusResponse, error) {
			entered <- struct{}{}
			select {
			case <-release:
				return &clientv3.StatusResponse{}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}}
	}
	var created atomic.Int32
	factory := func(clientv3.Config) (*clientv3.Client, error) { return clients[created.Add(1)-1], nil }
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- c.initClient(ctx, factory) }()
	}
	receive(t, ctx, entered)
	receive(t, ctx, entered)
	close(release)
	require.NoError(t, receive(t, ctx, results))
	require.NoError(t, receive(t, ctx, results))
	closed := 0
	for _, cli := range clients {
		if cli.Ctx().Err() != nil {
			closed++
		}
	}
	require.Equal(t, 1, closed, "the losing Init must close its client")
	c.Close()
	for _, cli := range clients {
		require.ErrorIs(t, cli.Ctx().Err(), context.Canceled)
	}
}

func TestEtcdClient_UnitCloseDuringInit(t *testing.T) {
	ctx := unitContext(t)
	c := NewEtcdClient("fake", "/ROOT", "", "")
	cli := clientv3.NewCtxClient(ctx)
	entered, release := make(chan struct{}), make(chan struct{})
	cli.Maintenance = &stubMaintenance{status: func(context.Context, string) (*clientv3.StatusResponse, error) {
		close(entered)
		select {
		case <-release:
			return &clientv3.StatusResponse{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	result := make(chan error, 1)
	go func() {
		result <- c.initClient(ctx, func(clientv3.Config) (*clientv3.Client, error) { return cli, nil })
	}()
	receive(t, ctx, entered)
	c.Close()
	close(release)
	require.ErrorIs(t, receive(t, ctx, result), ErrClientClosed)
	require.ErrorIs(t, cli.Ctx().Err(), context.Canceled)
}

func TestEtcdClient_UnitUnavailable(t *testing.T) {
	calls := []struct {
		name string
		call func(*EtcdClient) error
	}{
		{"exists", func(c *EtcdClient) error { _, err := c.IsNodeExist(t.Context(), "GATE/1"); return err }},
		{"get", func(c *EtcdClient) error { _, err := c.GetKey(t.Context(), "GATE/1"); return err }},
		{"put", func(c *EtcdClient) error { return c.PutKey(t.Context(), "GATE/1", "value", 0) }},
		{"delete", func(c *EtcdClient) error { return c.DelKey(t.Context(), "GATE/1") }},
		{"delete directory", func(c *EtcdClient) error { _, err := c.DelDir(t.Context(), "GATE"); return err }},
		{"get node", func(c *EtcdClient) error { _, err := c.GetNode(t.Context(), "GATE/1"); return err }},
		{"put node", func(c *EtcdClient) error { return c.PutNode(t.Context(), "GATE/1", NodeInfo{}, 0) }},
		{"list", func(c *EtcdClient) error { _, _, err := c.ListNodes(t.Context(), "GATE"); return err }},
		{"list filtered", func(c *EtcdClient) error { _, _, err := c.ListNodesWith(t.Context(), "GATE", nil); return err }},
		{"grant", func(c *EtcdClient) error { _, err := c.GrantLease(t.Context(), 5); return err }},
		{"ttl", func(c *EtcdClient) error { _, err := c.GetLeaseTTL(t.Context(), 7); return err }},
		{"revoke", func(c *EtcdClient) error { return c.RevokeLease(t.Context(), 7) }},
		{"register", func(c *EtcdClient) error { _, err := c.RegisterNode(t.Context(), "GATE/1", NodeInfo{}, 5); return err }},
		{"keeper", func(c *EtcdClient) error {
			_, err := c.RegisterAndKeepAlive(t.Context(), "GATE/1", NodeInfo{}, 5)
			return err
		}},
	}
	for _, test := range calls {
		t.Run(test.name, func(t *testing.T) {
			c := NewEtcdClient("fake", "/ROOT", "", "")
			require.ErrorIs(t, test.call(c), ErrClientNotInitialized)
			c.Close()
			require.ErrorIs(t, test.call(c), ErrClientClosed)
		})
	}
}

func TestEtcdClient_UnitKeyOperations(t *testing.T) {
	c, _, kv, _ := fakeEtcdClient(t)
	ctx := unitContext(t)
	kv.get = func(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
		assert.Equal(t, "/ROOT/GATE/1", key)
		_, bounded := ctx.Deadline()
		assert.True(t, bounded)
		op := clientv3.OpGet(key, opts...)
		if op.IsCountOnly() {
			return &clientv3.GetResponse{Count: 1}, nil
		}
		return &clientv3.GetResponse{Count: 1, Kvs: []*mvccpb.KeyValue{{Value: []byte(`{"Type":"GATE","ID":1}`)}}}, nil
	}
	found, err := c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.True(t, found)
	value, err := c.GetKey(ctx, "GATE/1")
	require.NoError(t, err)
	require.JSONEq(t, `{"Type":"GATE","ID":1}`, string(value))
	node, err := c.GetNode(ctx, "GATE/1")
	require.NoError(t, err)
	require.Equal(t, uint32(1), node.GetID())
	require.IsType(t, json.Number(""), node[NodeKeyID])
	for _, leaseID := range []int64{0, 7} {
		kv.put = func(ctx context.Context, key, value string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
			expected := clientv3.OpPut("/ROOT/GATE/1", value)
			if leaseID != 0 {
				expected = clientv3.OpPut("/ROOT/GATE/1", value, clientv3.WithLease(clientv3.LeaseID(leaseID)))
			}
			assert.Equal(t, expected, clientv3.OpPut(key, value, opts...))
			return &clientv3.PutResponse{}, nil
		}
		require.NoError(t, c.PutKey(ctx, "GATE/1", "value", leaseID))
		require.NoError(t, c.PutNode(ctx, "GATE/1", node, leaseID))
	}
	kv.get = nil
	found, err = c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.False(t, found)
	value, err = c.GetKey(ctx, "GATE/1")
	require.NoError(t, err)
	require.Nil(t, value)
	node, err = c.GetNode(ctx, "GATE/1")
	require.NoError(t, err)
	require.Nil(t, node)
	kv.del = func(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
		return &clientv3.DeleteResponse{}, nil
	}
	require.ErrorIs(t, c.DelKey(ctx, "GATE/1"), ErrNoKeyDeleted)
}

func TestEtcdClient_UnitRPCErrors(t *testing.T) {
	c, _, kv, lease := fakeEtcdClient(t)
	failure := errors.New("RPC failed")
	kv.get = func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
		return nil, failure
	}
	kv.put = func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error) {
		return nil, failure
	}
	kv.del = func(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
		return nil, failure
	}
	lease.grant = func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) { return nil, failure }
	lease.ttl = func(context.Context, clientv3.LeaseID) (*clientv3.LeaseTimeToLiveResponse, error) {
		return nil, failure
	}
	lease.revoke = func(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) { return nil, failure }
	_, err := c.IsNodeExist(t.Context(), "GATE/1")
	require.ErrorIs(t, err, failure)
	_, err = c.GetKey(t.Context(), "GATE/1")
	require.ErrorIs(t, err, failure)
	_, err = c.GetNode(t.Context(), "GATE/1")
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, c.PutKey(t.Context(), "GATE/1", "value", 0), failure)
	require.ErrorIs(t, c.PutNode(t.Context(), "GATE/1", NodeInfo{}, 0), failure)
	require.ErrorIs(t, c.DelKey(t.Context(), "GATE/1"), failure)
	_, err = c.DelDir(t.Context(), "GATE")
	require.ErrorIs(t, err, failure)
	_, _, err = c.ListNodes(t.Context(), "GATE")
	require.ErrorIs(t, err, failure)
	_, err = c.GrantLease(t.Context(), 5)
	require.ErrorIs(t, err, failure)
	_, err = c.GetLeaseTTL(t.Context(), 7)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, c.RevokeLease(t.Context(), 7), failure)
	_, err = c.RegisterNode(t.Context(), "GATE/1", NodeInfo{}, 5)
	require.ErrorIs(t, err, failure)
}

func TestEtcdClient_UnitInvalidNode(t *testing.T) {
	c, _, kv, _ := fakeEtcdClient(t)
	kv.get = func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
		return &clientv3.GetResponse{Count: 1, Kvs: []*mvccpb.KeyValue{{Key: []byte("/ROOT/GATE/1"), Value: []byte(`{"Type":"GATE","ID":{}}`)}}}, nil
	}
	_, err := c.GetNode(t.Context(), "GATE/1")
	require.Error(t, err)
	_, _, err = c.ListNodes(t.Context(), "GATE")
	require.ErrorContains(t, err, "/ROOT/GATE/1")
	var unsupported *json.UnsupportedTypeError
	require.ErrorAs(t, c.PutNode(t.Context(), "GATE/1", make(chan int), 0), &unsupported)
	_, err = c.GetKey(t.Context(), "../other/key")
	require.Error(t, err)
}

func TestEtcdClient_UnitDirectoryBoundaries(t *testing.T) {
	for _, dir := range []string{"GATE", "GATE/"} {
		t.Run(dir, func(t *testing.T) {
			c, cli, kv, _ := fakeEtcdClient(t)
			ctx := unitContext(t)
			check := func(op clientv3.Op) {
				assert.Equal(t, "/ROOT/GATE/", string(op.KeyBytes()))
				assert.Equal(t, clientv3.GetPrefixRangeEnd("/ROOT/GATE/"), string(op.RangeBytes()))
			}
			kv.get = func(_ context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
				check(clientv3.OpGet(key, opts...))
				return &clientv3.GetResponse{Header: &pb.ResponseHeader{Revision: 100}}, nil
			}
			nodes, rev, err := c.ListNodes(ctx, dir)
			require.NoError(t, err)
			require.Nil(t, nodes)
			require.Equal(t, int64(100), rev, "empty snapshot must preserve revision")
			_, rev, err = c.ListNodesWith(ctx, dir, []string{"GATE"})
			require.NoError(t, err)
			require.Equal(t, int64(100), rev)
			kv.del = func(_ context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
				check(clientv3.OpDelete(key, opts...))
				return &clientv3.DeleteResponse{Deleted: 2}, nil
			}
			count, err := c.DelDir(ctx, dir)
			require.NoError(t, err)
			require.Equal(t, int64(2), count)
			source := make(chan clientv3.WatchResponse)
			close(source)
			cli.Watcher = &stubWatcher{watch: func(_ context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
				op := clientv3.OpGet(key, opts...)
				check(op)
				assert.Equal(t, int64(101), op.Rev())
				return source
			}}
			_, open := <-c.WatchDir(ctx, dir, rev+1)
			require.False(t, open)
		})
	}
}

func TestEtcdClient_UnitListNodes(t *testing.T) {
	c, _, kv, _ := fakeEtcdClient(t)
	kv.get = func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
		return &clientv3.GetResponse{Header: &pb.ResponseHeader{Revision: 123}, Count: 2, Kvs: []*mvccpb.KeyValue{
			{Key: []byte("/ROOT/GATE/1"), Value: []byte(`{"Type":"GATE","ID":1}`)},
			{Key: []byte("/ROOT/GAME/1"), Value: []byte(`{"Type":"GAME","ID":1}`)},
		}}, nil
	}
	dependencies := []string{"GAME", "GATE"}
	nodes, rev, err := c.ListNodesWith(t.Context(), "/", dependencies[:1])
	require.NoError(t, err)
	require.Equal(t, int64(123), rev)
	require.Len(t, nodes, 1)
	require.Equal(t, "GAME", nodes[0].GetType())
	require.Equal(t, []string{"GAME", "GATE"}, dependencies)
}

func TestEtcdClient_UnitRegisterNode(t *testing.T) {
	c, _, kv, lease := fakeEtcdClient(t)
	transaction := &stubTxn{}
	kv.txn = func(ctx context.Context) clientv3.Txn {
		_, bounded := ctx.Deadline()
		assert.True(t, bounded)
		return transaction
	}
	id, err := c.RegisterNode(t.Context(), "GATE/1", NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1)}, 5)
	require.NoError(t, err)
	require.Equal(t, int64(7), id)
	require.Len(t, transaction.cmps, 1)
	cmp := transaction.cmps[0].GetCompare()
	require.Equal(t, pb.Compare_CREATE, cmp.Target)
	require.Equal(t, pb.Compare_EQUAL, cmp.Result)
	require.Equal(t, int64(0), cmp.GetCreateRevision())
	require.Equal(t, "/ROOT/GATE/1", string(cmp.Key))
	require.Len(t, transaction.ops, 1)
	require.Equal(t, clientv3.OpPut("/ROOT/GATE/1", `{"ID":1,"Type":"GATE"}`, clientv3.WithLease(7)), transaction.ops[0])
	var grants int
	lease.grant = func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) { grants++; return nil, nil }
	_, err = c.RegisterNode(t.Context(), "GATE/1", make(chan int), 5)
	require.Error(t, err)
	_, err = c.RegisterNode(t.Context(), "GATE/1", NodeInfo{}, 0)
	require.Error(t, err)
	_, err = c.RegisterNode(t.Context(), "../GATE/1", NodeInfo{}, 5)
	require.Error(t, err)
	require.Zero(t, grants, "invalid requests must not allocate leases")
}

func TestEtcdClient_UnitRegisterRollback(t *testing.T) {
	failure := errors.New("transaction failed")
	rollbackFailure := errors.New("rollback failed")
	for _, test := range []struct {
		name                  string
		commitErr, cleanupErr error
		cancel                bool
	}{
		{"conflict", nil, nil, false},
		{"transaction error", failure, nil, false},
		{"cancelled transaction", context.Canceled, nil, true},
		{"rollback error", failure, rollbackFailure, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, _, kv, lease := fakeEtcdClient(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			kv.txn = func(context.Context) clientv3.Txn {
				return &stubTxn{commit: func() (*clientv3.TxnResponse, error) {
					if test.cancel {
						cancel()
					}
					return &clientv3.TxnResponse{Succeeded: false}, test.commitErr
				}}
			}
			var revokes int
			lease.revoke = func(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
				revokes++
				assert.Equal(t, clientv3.LeaseID(7), id)
				assert.NoError(t, ctx.Err(), "cleanup must not inherit cancellation")
				_, bounded := ctx.Deadline()
				assert.True(t, bounded)
				return nil, test.cleanupErr
			}
			id, err := c.RegisterNode(ctx, "GATE/1", NodeInfo{}, 5)
			require.Zero(t, id)
			cause := test.commitErr
			if cause == nil {
				cause = ErrNodeKeyExist
			}
			require.ErrorIs(t, err, cause)
			if test.cleanupErr != nil {
				require.ErrorIs(t, err, test.cleanupErr)
			}
			require.Equal(t, 1, revokes)
		})
	}
}

func TestEtcdClient_UnitLeaseOperations(t *testing.T) {
	c, _, _, lease := fakeEtcdClient(t)
	id, err := c.GrantLease(t.Context(), 5)
	require.NoError(t, err)
	require.Equal(t, int64(7), id)
	ttl, err := c.GetLeaseTTL(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, 5, ttl)
	require.NoError(t, c.RevokeLease(t.Context(), id))
	lease.revoke = func(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
		return nil, rpctypes.ErrLeaseNotFound
	}
	keeper := NewNodeAliveKeeper("GATE/1", NodeInfo{}, 5)
	keeper.leaseId = id
	require.NoError(t, keeper.Revoke(t.Context(), c), "an expired lease is already revoked")
	lease.grant = func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) { return nil, nil }
	_, err = c.GrantLease(t.Context(), 5)
	require.ErrorIs(t, err, ErrEmptyLease)
	lease.grant = func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
		return &clientv3.LeaseGrantResponse{}, nil
	}
	_, err = c.RegisterNode(t.Context(), "GATE/1", NodeInfo{}, 5)
	require.ErrorIs(t, err, ErrEmptyLease)
}

func TestDecodeEtcdNode(t *testing.T) {
	for _, input := range []string{"", "null", "[]", `{} {}`, `{}garbage`, `{"ID":{}}`, `{"ID":[]}`, `{"ID":null}`, `{"ID":true}`, `{"ID":-1}`, `{"ID":4294967296}`, `{"ID":1.5}`, `{"Type":42}`} {
		t.Run(input, func(t *testing.T) { _, err := decodeEtcdNode([]byte(input)); require.Error(t, err) })
	}
	for _, input := range []string{`{}`, `{"Type":"GATE","ID":4294967295}`, `{"ID":"12"}`} {
		t.Run(input, func(t *testing.T) { _, err := decodeEtcdNode([]byte(input)); require.NoError(t, err) })
	}
}

func TestParseNodeTypeAndID(t *testing.T) {
	for _, test := range []struct {
		key, nodeType string
		id            uint32
	}{
		{"/ROOT/GATE/1", "GATE", 1}, {"GATE/1", "GATE", 1}, {"/GATE/4294967295", "GATE", ^uint32(0)},
		{"/ROOT/GATE/-1", "", 0}, {"/ROOT/GATE/4294967297", "", 0}, {"/ROOT/GATE/0", "", 0},
		{"/ROOT/GATE/abc", "", 0}, {"/ROOT//1", "", 0}, {"/ROOT/GATE/", "", 0}, {"1", "", 0}, {"/ROOT/../1", "", 0},
	} {
		t.Run(test.key, func(t *testing.T) {
			typ, id := parseNodeTypeAndID(test.key)
			require.Equal(t, test.nodeType, typ)
			require.Equal(t, test.id, id)
		})
	}
}

func watchEvent(kind NodeEventType) *clientv3.Event {
	event := &clientv3.Event{Kv: &mvccpb.KeyValue{Key: []byte("/ROOT/GATE/1"), Value: []byte(`{"Type":"GATE","ID":1}`), CreateRevision: 1, ModRevision: 2, Version: 2}}
	if kind == EventCreate {
		event.Kv.ModRevision, event.Kv.Version = 1, 1
	}
	if kind == EventDelete {
		event.Type, event.Kv.Value = mvccpb.DELETE, nil
	}
	return event
}

func TestPropagateWatchEvent(t *testing.T) {
	for _, kind := range []NodeEventType{EventCreate, EventUpdate, EventDelete} {
		t.Run(kind.String(), func(t *testing.T) {
			output := make(chan *NodeEvent, 1)
			require.NoError(t, propagateWatchEvent(t.Context(), output, watchEvent(kind)))
			event := <-output
			require.Equal(t, kind, event.EvType)
			require.Equal(t, "GATE", event.NodeType)
			require.Equal(t, uint32(1), event.NodeID)
			if kind == EventDelete {
				require.Nil(t, event.Info)
			} else {
				require.Equal(t, uint32(1), event.Info.GetID())
			}
		})
	}
	for _, value := range []string{`{"ID":{}}`, `{"Type":"GAME","ID":1}`, `{"Type":"GATE","ID":2}`, `{}`, `null`, `{} {}`} {
		t.Run(value, func(t *testing.T) {
			event := watchEvent(EventUpdate)
			event.Kv.Value = []byte(value)
			require.Error(t, propagateWatchEvent(t.Context(), make(chan *NodeEvent, 1), event))
		})
	}
	require.Error(t, propagateWatchEvent(t.Context(), nil, nil))
	invalid := watchEvent(EventUpdate)
	invalid.Kv.Key = []byte("/ROOT/config")
	require.NoError(t, propagateWatchEvent(t.Context(), nil, invalid), "ordinary keys are ignored")
}

func TestEtcdClient_UnitWatchBackpressure(t *testing.T) {
	c, cli, _, _ := fakeEtcdClient(t)
	ctx := unitContext(t)
	source := make(chan clientv3.WatchResponse, 1)
	source <- clientv3.WatchResponse{Events: []*clientv3.Event{watchEvent(EventCreate), watchEvent(EventUpdate), watchEvent(EventDelete)}}
	close(source)
	cli.Watcher = &stubWatcher{watch: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return source }}
	output := make(chan *NodeEvent) // 每个事件均需等待消费者，UPDATE 也不可丢弃。
	done := c.watch(ctx, "GATE", 0, output, true, true)
	for _, kind := range []NodeEventType{EventCreate, EventUpdate, EventDelete} {
		require.Equal(t, kind, receive(t, ctx, output).EvType)
	}
	receive(t, ctx, done)
	_, open := <-output
	require.False(t, open)
}

func TestEtcdClient_UnitWatchCancellation(t *testing.T) {
	for _, kind := range []NodeEventType{EventCreate, EventUpdate, EventDelete} {
		t.Run(kind.String(), func(t *testing.T) {
			c, cli, _, _ := fakeEtcdClient(t)
			ctx := unitContext(t)
			watchCtx, cancel := context.WithCancel(ctx)
			source := make(chan clientv3.WatchResponse)
			cli.Watcher = &stubWatcher{watch: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return source }}
			output := make(chan *NodeEvent, 1)
			output <- &NodeEvent{}
			done := c.watch(watchCtx, "GATE", 0, output, true, true)
			select {
			case source <- clientv3.WatchResponse{Events: []*clientv3.Event{watchEvent(kind)}}:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			cancel()
			receive(t, ctx, done)
			<-output
			_, open := <-output
			require.False(t, open)
		})
	}
}

func TestEtcdClient_UnitWatchTermination(t *testing.T) {
	for _, response := range []clientv3.WatchResponse{{CompactRevision: 100}, {Canceled: true}, {Events: []*clientv3.Event{{}}}} {
		t.Run(fmt.Sprint(response), func(t *testing.T) {
			c, cli, _, _ := fakeEtcdClient(t)
			ctx := unitContext(t)
			source := make(chan clientv3.WatchResponse, 1)
			source <- response
			cli.Watcher = &stubWatcher{watch: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan { return source }}
			output := c.WatchDir(ctx, "/", 1)
			select {
			case _, open := <-output:
				require.False(t, open)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
	t.Run("caller owns Watch output", func(t *testing.T) {
		c, cli, _, _ := fakeEtcdClient(t)
		ctx := unitContext(t)
		source := make(chan clientv3.WatchResponse)
		close(source)
		cli.Watcher = &stubWatcher{watch: func(_ context.Context, key string, _ ...clientv3.OpOption) clientv3.WatchChan {
			assert.Equal(t, "/ROOT/GATE", key)
			return source
		}}
		output := make(chan *NodeEvent)
		receive(t, ctx, c.watch(ctx, "GATE", 0, output, false, false))
		select {
		case <-output:
			t.Fatal("Watch must not close the caller's channel")
		default:
		}
		close(output)
	})
	t.Run("unavailable client", func(t *testing.T) {
		c := NewEtcdClient("fake", "/ROOT", "", "")
		_, open := <-c.WatchDir(t.Context(), "/", 0)
		require.False(t, open)
	})
}

func TestEtcdClient_UnitWatchPublicAPI(t *testing.T) {
	c, cli, _, _ := fakeEtcdClient(t)
	ctx := unitContext(t)
	started := make(chan struct{})
	cli.Watcher = &stubWatcher{watch: func(_ context.Context, key string, _ ...clientv3.OpOption) clientv3.WatchChan {
		assert.Equal(t, "/ROOT/GATE", key)
		close(started)
		return make(chan clientv3.WatchResponse)
	}}
	output := make(chan *NodeEvent)
	c.Watch(ctx, "GATE", 0, output)
	receive(t, ctx, started)
	c.Close()
	select {
	case <-output:
		t.Fatal("Watch must preserve caller ownership of the channel")
	default:
	}
	close(output)
}

func TestNodeAliveKeeper_KeepAliveFailure(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure error
	}{{"error", errors.New("keepalive failed")}, {"nil channel", nil}} {
		t.Run(test.name, func(t *testing.T) {
			c, _, _, lease := fakeEtcdClient(t)
			lease.keep = func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
				return nil, test.failure
			}
			var revoked int
			lease.revoke = func(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
				revoked++
				assert.NoError(t, ctx.Err())
				assert.Equal(t, clientv3.LeaseID(7), id)
				return &clientv3.LeaseRevokeResponse{}, nil
			}
			keeper, err := c.RegisterAndKeepAlive(t.Context(), "GATE/1", NodeInfo{}, 5)
			require.Error(t, err)
			if test.failure != nil {
				require.ErrorIs(t, err, test.failure)
			}
			require.Nil(t, keeper)
			require.Equal(t, 1, revoked)
		})
	}
}

func TestNodeAliveKeeper_RevokeAndCancellation(t *testing.T) {
	c, _, _, lease := fakeEtcdClient(t)
	ctx := unitContext(t)
	keepContexts := make(chan context.Context, 1)
	lease.keep = func(ctx context.Context, _ clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
		keepContexts <- ctx
		return make(chan *clientv3.LeaseKeepAliveResponse), nil
	}
	var revokes atomic.Int32
	lease.revoke = func(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
		revokes.Add(1)
		assert.Equal(t, clientv3.LeaseID(7), id)
		return &clientv3.LeaseRevokeResponse{}, ctx.Err()
	}
	keeperCtx, cancel := context.WithCancel(ctx)
	keeper, err := c.RegisterAndKeepAlive(keeperCtx, "GATE/1", NodeInfo{}, 5)
	require.NoError(t, err)
	keepCtx := receive(t, ctx, keepContexts)
	require.NoError(t, keeper.RegisterNode(ctx, c), "healthy registration is idempotent")
	cancel()
	receive(t, ctx, keeper.done)
	receive(t, ctx, keepCtx.Done())
	require.False(t, keeper.alive.Load())
	require.NoError(t, keeper.Revoke(ctx, c), "lease must be revoked even when alive is false")
	require.NoError(t, keeper.Revoke(ctx, c))
	require.Equal(t, int32(1), revokes.Load())
	require.ErrorIs(t, keeper.RegisterNode(ctx, c), errKeeperStopped)
}

func TestNodeAliveKeeper_RevokeStopsWorker(t *testing.T) {
	c, _, _, lease := fakeEtcdClient(t)
	ctx := unitContext(t)
	var grants atomic.Int32
	lease.grant = func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
		grants.Add(1)
		return &clientv3.LeaseGrantResponse{ID: 7}, nil
	}
	keeper, err := c.RegisterAndKeepAlive(ctx, "GATE/1", NodeInfo{}, 5)
	require.NoError(t, err)
	require.NoError(t, keeper.Revoke(ctx, c))
	receive(t, ctx, keeper.done)
	require.Equal(t, int32(1), grants.Load())
	require.False(t, keeper.alive.Load())
}

func TestNodeAliveKeeper_RevokeDuringRegistration(t *testing.T) {
	c, _, kv, lease := fakeEtcdClient(t)
	ctx := unitContext(t)
	entered := make(chan struct{})
	kv.txn = func(ctx context.Context) clientv3.Txn {
		return &stubTxn{commit: func() (*clientv3.TxnResponse, error) {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}}
	}
	var revokes atomic.Int32
	lease.revoke = func(ctx context.Context, _ clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
		revokes.Add(1)
		assert.NoError(t, ctx.Err())
		return &clientv3.LeaseRevokeResponse{}, nil
	}
	keeper := NewNodeAliveKeeper("GATE/1", NodeInfo{}, 5)
	result := make(chan error, 1)
	go func() { result <- keeper.RegisterNode(ctx, c) }()
	receive(t, ctx, entered)
	require.Error(t, keeper.RegisterNode(ctx, c), "overlapping registration must not allocate another lease")
	require.NoError(t, keeper.Revoke(ctx, c))
	require.ErrorIs(t, receive(t, ctx, result), context.Canceled)
	require.Equal(t, int32(1), revokes.Load())
	require.False(t, keeper.alive.Load())
}

func TestNodeAliveKeeper_RevokeCancelledContext(t *testing.T) {
	c, _, kv, _ := fakeEtcdClient(t)
	ctx := unitContext(t)
	entered, release := make(chan struct{}), make(chan struct{})
	kv.txn = func(ctx context.Context) clientv3.Txn {
		return &stubTxn{commit: func() (*clientv3.TxnResponse, error) {
			close(entered)
			select {
			case <-release:
				return nil, ctx.Err()
			case <-t.Context().Done():
				return nil, t.Context().Err()
			}
		}}
	}
	keeper := NewNodeAliveKeeper("GATE/1", NodeInfo{}, 5)
	registered := make(chan error, 1)
	go func() { registered <- keeper.RegisterNode(ctx, c) }()
	receive(t, ctx, entered)
	revokeCtx, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, keeper.Revoke(revokeCtx, c), context.Canceled)
	close(release)
	require.ErrorIs(t, receive(t, ctx, registered), context.Canceled)
	require.NoError(t, keeper.Revoke(ctx, c))
}

func TestNodeAliveKeeper_Reregister(t *testing.T) {
	c, cli, kv, lease := fakeEtcdClient(t)
	ctx := unitContext(t)
	var grants atomic.Int32
	lease.grant = func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
		return &clientv3.LeaseGrantResponse{ID: clientv3.LeaseID(grants.Add(1))}, nil
	}
	responses := []chan *clientv3.LeaseKeepAliveResponse{make(chan *clientv3.LeaseKeepAliveResponse), make(chan *clientv3.LeaseKeepAliveResponse)}
	keepContexts := make(chan context.Context, 2)
	lease.keep = func(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
		keepContexts <- ctx
		return responses[int(id)-1], nil
	}
	var payloads []string
	kv.txn = func(context.Context) clientv3.Txn {
		transaction := &stubTxn{}
		transaction.commit = func() (*clientv3.TxnResponse, error) {
			payloads = append(payloads, string(transaction.ops[0].ValueBytes()))
			return &clientv3.TxnResponse{Succeeded: true}, nil
		}
		return transaction
	}
	value := NodeInfo{NodeKeyType: "GATE", NodeKeyID: int64(1)}
	keeper := NewNodeAliveKeeper("GATE/1", value, 5)
	require.NoError(t, keeper.RegisterNode(ctx, c))
	firstCtx := receive(t, ctx, keepContexts)
	value.SetString(NodeKeyType, "changed after registration")
	keeper.done = make(chan struct{})
	ticks := make(chan time.Time)
	require.NoError(t, c.launch(func(cli *clientv3.Client) { keeper.run(ctx, c, cli, ticks) }))
	close(responses[0])
	receive(t, ctx, firstCtx.Done()) // 保活丢失已被 worker 处理。
	select {
	case ticks <- time.Now():
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	receive(t, ctx, keepContexts)
	keeper.mu.Lock()
	registration := keeper.registerDone
	keeper.mu.Unlock()
	if registration != nil {
		receive(t, ctx, registration)
	}
	require.Equal(t, int32(2), grants.Load())
	require.Len(t, payloads, 2)
	require.Equal(t, payloads[0], payloads[1], "automatic registration uses the original payload")
	require.NoError(t, keeper.Revoke(ctx, c))
	require.NoError(t, cli.Ctx().Err())
}

func TestNodeAliveKeeper_CancelDuringExternalRegistration(t *testing.T) {
	c, _, kv, lease := fakeEtcdClient(t)
	ctx := unitContext(t)
	var grants atomic.Int32
	lease.grant = func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
		return &clientv3.LeaseGrantResponse{ID: clientv3.LeaseID(grants.Add(1))}, nil
	}
	responses := make(chan *clientv3.LeaseKeepAliveResponse)
	keepContexts := make(chan context.Context, 1)
	lease.keep = func(ctx context.Context, _ clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
		keepContexts <- ctx
		return responses, nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	keeper, err := c.RegisterAndKeepAlive(workerCtx, "GATE/1", NodeInfo{}, 5)
	require.NoError(t, err)
	firstCtx := receive(t, ctx, keepContexts)
	close(responses)
	receive(t, ctx, firstCtx.Done())
	entered := make(chan struct{})
	kv.txn = func(ctx context.Context) clientv3.Txn {
		return &stubTxn{commit: func() (*clientv3.TxnResponse, error) {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}}
	}
	result := make(chan error, 1)
	go func() { result <- keeper.RegisterNode(ctx, c) }()
	receive(t, ctx, entered)
	cancel()
	receive(t, ctx, keeper.done)
	require.ErrorIs(t, receive(t, ctx, result), context.Canceled)
	require.False(t, keeper.alive.Load())
	require.NoError(t, keeper.Revoke(ctx, c))
}

func TestNodeAliveKeeper_RevokeRetry(t *testing.T) {
	c, _, _, lease := fakeEtcdClient(t)
	ctx := unitContext(t)
	keeper := NewNodeAliveKeeper("GATE/1", NodeInfo{}, 5)
	require.NoError(t, keeper.RegisterNode(ctx, c))
	failure := errors.New("revoke unavailable")
	lease.revoke = func(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) { return nil, failure }
	require.ErrorIs(t, keeper.Revoke(ctx, c), failure)
	lease.revoke = func(_ context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
		assert.Equal(t, clientv3.LeaseID(7), id, "failed revoke must retain the lease ID")
		return &clientv3.LeaseRevokeResponse{}, nil
	}
	require.NoError(t, keeper.Revoke(ctx, c))
}

func TestEtcdClient_UnitConcurrentClose(t *testing.T) {
	c, cli, _, _ := fakeEtcdClient(t)
	ctx := unitContext(t)
	watchStarted := make(chan struct{})
	cli.Watcher = &stubWatcher{watch: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan {
		close(watchStarted)
		return make(chan clientv3.WatchResponse)
	}}
	output := c.WatchDir(ctx, "/", 0)
	receive(t, ctx, watchStarted)
	keeper, err := c.RegisterAndKeepAlive(ctx, "GATE/1", NodeInfo{}, 5)
	require.NoError(t, err)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if worker%2 == 0 {
				c.Close()
				return
			}
			for range 64 {
				c.SetVerbose(0)
				_, err := c.GetKey(ctx, "GATE/1")
				if err != nil && !errors.Is(err, ErrClientClosed) {
					t.Errorf("unexpected get error: %v", err)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	receive(t, ctx, keeper.done)
	require.False(t, keeper.alive.Load())
	_, open := <-output
	require.False(t, open)
	require.ErrorIs(t, cli.Ctx().Err(), context.Canceled)
}
