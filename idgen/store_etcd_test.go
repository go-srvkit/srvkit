// Copyright © 2026 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package idgen

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type etcdCounterTestServer struct {
	etcdserverpb.UnimplementedKVServer
	rangeFn func(context.Context, *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error)
	putFn   func(context.Context, *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error)
}

func (s *etcdCounterTestServer) Range(ctx context.Context, req *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
	if s.rangeFn != nil {
		return s.rangeFn(ctx, req)
	}
	return &etcdserverpb.RangeResponse{Header: &etcdserverpb.ResponseHeader{Revision: 1}}, nil
}

func (s *etcdCounterTestServer) Put(ctx context.Context, req *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
	return s.putFn(ctx, req)
}

func etcdCounterTestConfig(t *testing.T, kv *etcdCounterTestServer) clientv3.Config {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	etcdserverpb.RegisterKVServer(server, kv)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		if err := listener.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
		if err := <-done; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("serve: %v", err)
		}
	})
	return clientv3.Config{
		Endpoints: []string{"passthrough:///etcd-counter-test"},
		Logger:    zap.NewNop(),
		DialOptions: []grpc.DialOption{
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return listener.DialContext(ctx)
			}),
		},
	}
}

func newEtcdCounterForTest(t *testing.T, config clientv3.Config) *EtcdCounter {
	t.Helper()
	store, err := NewEtcdCounter(config, "/idgen/test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close counter: %v", err)
		}
	})
	return store
}

func etcdCounterTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestEtcdCounterRevision(t *testing.T) {
	revisions := []int64{10, 14, 25}
	var reads atomic.Int64
	var writes atomic.Int64
	server := &etcdCounterTestServer{
		rangeFn: func(_ context.Context, req *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
			reads.Add(1)
			if string(req.Key) != "/idgen/test" || req.Serializable || !req.CountOnly {
				return nil, status.Error(codes.InvalidArgument, "expected linearizable key check")
			}
			// A read revision is not an allocation and must never be returned by Incr.
			return &etcdserverpb.RangeResponse{Header: &etcdserverpb.ResponseHeader{Revision: 999}}, nil
		},
		putFn: func(_ context.Context, req *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
			if string(req.Key) != "/idgen/test" || string(req.Value) != "allocate" || req.Lease != 0 {
				return nil, status.Error(codes.InvalidArgument, "unexpected allocation request")
			}
			i := writes.Add(1) - 1
			return &etcdserverpb.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: revisions[i]}}, nil
		},
	}
	store := newEtcdCounterForTest(t, etcdCounterTestConfig(t, server))
	ctx := etcdCounterTestContext(t)
	if id, err := store.Incr(ctx); id != 0 || !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("before Init: id=%d err=%v", id, err)
	}
	for i := 0; i < 2; i++ {
		if err := store.Init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if reads.Load() != 1 || writes.Load() != 0 {
		t.Fatalf("Init consumed revisions or repeated requests: reads=%d writes=%d", reads.Load(), writes.Load())
	}
	for _, want := range revisions {
		got, err := store.Incr(ctx)
		if err != nil || got != want {
			t.Fatalf("Incr: got=%d err=%v want=%d", got, err, want)
		}
	}
	if reads.Load() != 1 || writes.Load() != int64(len(revisions)) {
		t.Fatalf("expected one independent Put per allocation: reads=%d writes=%d", reads.Load(), writes.Load())
	}
}

func TestEtcdCounterInvalidConfig(t *testing.T) {
	if store, err := NewEtcdCounter(clientv3.Config{}, "/idgen/test"); store != nil || !errors.Is(err, clientv3.ErrNoAvailableEndpoints) {
		t.Fatalf("missing endpoints: store=%v err=%v", store, err)
	}
	if store, err := NewEtcdCounter(clientv3.Config{Endpoints: []string{"localhost:2379"}}, ""); store != nil || err == nil {
		t.Fatalf("empty key: store=%v err=%v", store, err)
	}
}

func TestEtcdCounterInitFailure(t *testing.T) {
	var reads atomic.Int64
	server := &etcdCounterTestServer{
		rangeFn: func(context.Context, *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
			if reads.Add(1) == 1 {
				return nil, status.Error(codes.PermissionDenied, "denied")
			}
			return &etcdserverpb.RangeResponse{}, nil
		},
	}
	store := newEtcdCounterForTest(t, etcdCounterTestConfig(t, server))
	ctx := etcdCounterTestContext(t)
	if err := store.Init(ctx); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Init: %v", err)
	}
	if id, err := store.Incr(ctx); id != 0 || !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("after failed Init: id=%d err=%v", id, err)
	}
	if err := store.Init(ctx); err != nil {
		t.Fatalf("retry Init: %v", err)
	}
}

func TestEtcdCounterPutFailure(t *testing.T) {
	tests := []struct {
		name string
		resp *etcdserverpb.PutResponse
		err  error
	}{
		{name: "permission denied", err: status.Error(codes.PermissionDenied, "denied")},
		{name: "missing header", resp: &etcdserverpb.PutResponse{}},
		{name: "zero revision", resp: &etcdserverpb.PutResponse{Header: &etcdserverpb.ResponseHeader{}}},
		{name: "negative revision", resp: &etcdserverpb.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: -1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &etcdCounterTestServer{
				putFn: func(context.Context, *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
					return tt.resp, tt.err
				},
			}
			store := newEtcdCounterForTest(t, etcdCounterTestConfig(t, server))
			ctx := etcdCounterTestContext(t)
			if err := store.Init(ctx); err != nil {
				t.Fatal(err)
			}
			id, err := store.Incr(ctx)
			if id != 0 || err == nil {
				t.Fatalf("Incr: id=%d err=%v", id, err)
			}
			if tt.err != nil {
				if status.Code(err) != codes.PermissionDenied {
					t.Fatalf("lost underlying RPC error: %v", err)
				}
			} else if !errors.Is(err, ErrCounterOutOfRange) {
				t.Fatalf("expected range error: %v", err)
			}
		})
	}
}

func TestEtcdCounterClose(t *testing.T) {
	var zero EtcdCounter
	if err := zero.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := etcdCounterTestContext(t)
	if err := zero.Init(ctx); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("zero Init: %v", err)
	}
	if id, err := zero.Incr(ctx); id != 0 || !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("zero Incr: id=%d err=%v", id, err)
	}
	store := newEtcdCounterForTest(t, etcdCounterTestConfig(t, &etcdCounterTestServer{}))
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	client := store.client
	for i := 0; i < 2; i++ {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if client.Ctx().Err() == nil {
		t.Fatal("Close did not cancel the owned client")
	}
	if err := store.Init(ctx); !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("closed Init: %v", err)
	}
	if id, err := store.Incr(ctx); id != 0 || !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("closed Incr: id=%d err=%v", id, err)
	}
}

func TestEtcdCounterCancelAndCloseInFlight(t *testing.T) {
	for _, action := range []string{"cancel", "close"} {
		t.Run(action, func(t *testing.T) {
			entered := make(chan struct{})
			server := &etcdCounterTestServer{
				putFn: func(ctx context.Context, _ *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
					close(entered)
					<-ctx.Done()
					return nil, status.FromContextError(ctx.Err()).Err()
				},
			}
			store := newEtcdCounterForTest(t, etcdCounterTestConfig(t, server))
			ctx := etcdCounterTestContext(t)
			if err := store.Init(ctx); err != nil {
				t.Fatal(err)
			}
			requestCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				id, err := store.Incr(requestCtx)
				if id != 0 {
					done <- errors.New("failed allocation returned a counter")
					return
				}
				done <- err
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("Put did not start")
			}
			if action == "cancel" {
				cancel()
			} else if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) && status.Code(err) != codes.Canceled {
					t.Fatalf("expected canceled allocation: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("allocation did not stop")
			}
		})
	}
}

func TestEtcdCounterCloseDuringInit(t *testing.T) {
	entered := make(chan struct{})
	server := &etcdCounterTestServer{
		rangeFn: func(ctx context.Context, _ *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
			close(entered)
			<-ctx.Done()
			return nil, status.FromContextError(ctx.Err()).Err()
		},
	}
	store := newEtcdCounterForTest(t, etcdCounterTestConfig(t, server))
	ctx := etcdCounterTestContext(t)
	done := make(chan error, 1)
	go func() { done <- store.Init(ctx) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("Init did not start")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Init succeeded after Close")
		}
	case <-ctx.Done():
		t.Fatal("Init did not stop")
	}
	if id, err := store.Incr(ctx); id != 0 || !errors.Is(err, ErrStoreClosed) {
		t.Fatalf("closed Incr: id=%d err=%v", id, err)
	}
}

func TestEtcdCounterConcurrentGenerators(t *testing.T) {
	var revision atomic.Int64
	server := &etcdCounterTestServer{
		putFn: func(context.Context, *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
			// Leave gaps to simulate writes from other etcd users.
			return &etcdserverpb.PutResponse{Header: &etcdserverpb.ResponseHeader{Revision: revision.Add(3)}}, nil
		},
	}
	config := etcdCounterTestConfig(t, server)
	ctx := etcdCounterTestContext(t)
	const workers = 8
	const count = DefaultSeqIDStep + 5 // Exercise segment reloads as well.
	ids := make(chan int64, workers*count)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	stores := make([]*EtcdCounter, 2)
	for i := range stores {
		store := newEtcdCounterForTest(t, config)
		if err := store.Init(ctx); err != nil {
			t.Fatal(err)
		}
		stores[i] = store
	}
	for i := 0; i < workers; i++ {
		// Exercise concurrent calls on shared stores and allocation across stores.
		gen := NewSegmentIDGen(stores[i%len(stores)])
		wg.Go(func() {
			for j := 0; j < count; j++ {
				id, err := gen.Next(ctx)
				if err != nil {
					errs <- err
					return
				}
				ids <- id
			}
		})
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := make(map[int64]struct{}, workers*count)
	for id := range ids {
		if _, exists := seen[id]; exists || id <= 0 {
			t.Fatalf("duplicate or invalid ID: %d", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != workers*count {
		t.Fatalf("got %d IDs, want %d", len(seen), workers*count)
	}
}
