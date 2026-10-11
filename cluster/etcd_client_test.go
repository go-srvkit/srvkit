// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 真实 etcd 集成测试仅在显式配置 TEST_ETCD_ADDR 时运行。
// 每个测试使用独立命名空间，避免污染已有数据或互相干扰。
func connectClient(t *testing.T, ctx context.Context) *EtcdClient {
	t.Helper()
	endpoints := os.Getenv("TEST_ETCD_ADDR")
	if endpoints == "" {
		t.Skip("integration test: set TEST_ETCD_ADDR to enable")
	}
	root := "/srvkit-tests/" + rand.Text()
	c := NewEtcdClient(endpoints, root, "", "")
	c.SetVerbose(0)
	require.NoError(t, c.Init(ctx))
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := c.DelDir(cleanupCtx, "")
		if err != nil {
			t.Errorf("cleanup etcd namespace: %v", err)
		}
		c.Close()
	})
	return c
}

func TestEtcdClient_IsNodeExist(t *testing.T) {
	ctx := unitContext(t)
	c := connectClient(t, ctx)
	found, err := c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, c.PutKey(ctx, "GATE/1", "value", 0))
	found, err = c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, c.DelKey(ctx, "GATE/1"))
	found, err = c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.False(t, found)
	require.ErrorIs(t, c.DelKey(ctx, "GATE/1"), ErrNoKeyDeleted)
}

func TestEtcdClient_PutNode(t *testing.T) {
	ctx := unitContext(t)
	c := connectClient(t, ctx)
	node := NewNodeFrom("GATE", 1)
	require.NoError(t, c.PutNode(ctx, "GATE/1", node, 0))
	stored, err := c.GetNode(ctx, "GATE/1")
	require.NoError(t, err)
	require.Equal(t, node.GetID(), stored.GetID())
	require.Equal(t, node.GetType(), stored.GetType())
	require.NoError(t, c.DelKey(ctx, "GATE/1"))
	stored, err = c.GetNode(ctx, "GATE/1")
	require.NoError(t, err)
	require.Nil(t, stored)
}

func TestEtcdClient_ListNodes(t *testing.T) {
	ctx := unitContext(t)
	c := connectClient(t, ctx)
	for i := 1; i <= 5; i++ {
		require.NoError(t, c.PutNode(ctx, fmt.Sprintf("services/GATE/%d", i), NewNodeFrom("GATE", uint32(i)), 0))
	}
	require.NoError(t, c.PutNode(ctx, "services-other/GATE/99", NewNodeFrom("GATE", 99), 0))
	nodes, rev, err := c.ListNodes(ctx, "services/")
	require.NoError(t, err)
	require.Len(t, nodes, 5)
	require.Positive(t, rev)
	count, err := c.DelDir(ctx, "services")
	require.NoError(t, err)
	require.Equal(t, int64(5), count)
	nodes, emptyRev, err := c.ListNodes(ctx, "services")
	require.NoError(t, err)
	require.Empty(t, nodes)
	require.Greater(t, emptyRev, rev)
	found, err := c.IsNodeExist(ctx, "services-other/GATE/99")
	require.NoError(t, err)
	require.True(t, found)
}

func TestEtcdClient_RegisterNode(t *testing.T) {
	ctx := unitContext(t)
	c := connectClient(t, ctx)
	node := NewNodeFrom("GATE", 1)
	id, err := c.RegisterNode(ctx, "GATE/1", node, 30)
	require.NoError(t, err)
	require.NotZero(t, id)
	other, err := c.RegisterNode(ctx, "GATE/1", node, 30)
	require.ErrorIs(t, err, ErrNodeKeyExist)
	require.Zero(t, other)
	ttl, err := c.GetLeaseTTL(ctx, id)
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.NoError(t, c.RevokeLease(ctx, id))
	found, err := c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.False(t, found)
}

func TestEtcdClient_RegisterAndKeepAlive(t *testing.T) {
	ctx := unitContext(t)
	c := connectClient(t, ctx)
	keeper, err := c.RegisterAndKeepAlive(ctx, "GATE/1", NewNodeFrom("GATE", 1), 30)
	require.NoError(t, err)
	found, err := c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, keeper.Revoke(ctx, c))
	receive(t, ctx, keeper.done)
	found, err = c.IsNodeExist(ctx, "GATE/1")
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, keeper.Revoke(ctx, c))
}

func TestEtcdClient_WatchDir(t *testing.T) {
	ctx := unitContext(t)
	c := connectClient(t, ctx)
	_, rev, err := c.ListNodes(ctx, "services")
	require.NoError(t, err)
	require.Positive(t, rev)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := c.WatchDir(watchCtx, "services", rev+1)
	require.NoError(t, c.PutNode(ctx, "services-other/GATE/99", NewNodeFrom("GATE", 99), 0))
	node := NewNodeFrom("GATE", 1)
	require.NoError(t, c.PutNode(ctx, "services/GATE/1", node, 0))
	event := receive(t, ctx, events)
	require.NotNil(t, event)
	require.Equal(t, EventCreate, event.EvType)
	require.Equal(t, uint32(1), event.NodeID)
	node.SetString(NodeKeyURI, "updated")
	require.NoError(t, c.PutNode(ctx, "services/GATE/1", node, 0))
	event = receive(t, ctx, events)
	require.NotNil(t, event)
	require.Equal(t, EventUpdate, event.EvType)
	require.Equal(t, "updated", event.Info.GetURI())
	require.NoError(t, c.DelKey(ctx, "services/GATE/1"))
	event = receive(t, ctx, events)
	require.NotNil(t, event)
	require.Equal(t, EventDelete, event.EvType)
	cancel()
	select {
	case _, open := <-events:
		require.False(t, open)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
