// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package cluster

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	etcdKeyspace = "/ROOT"
	nodeId       string
)

func connectClient(t *testing.T, ctx context.Context) *EtcdClient {
	var etcdHostAddr = os.Getenv("TEST_ETCD_ADDR")
	if etcdHostAddr == "" {
		etcdHostAddr = "127.0.0.1:2379"
	}
	var client = NewEtcdClient(etcdHostAddr, etcdKeyspace, "", "")
	client.verbose = VerboseLv2
	if err := client.Init(ctx); err != nil {
		t.Fatalf("connect server: %v", err)
	}
	return client
}

func TestEtcdClient_IsNodeExist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var client = connectClient(t, ctx)
	require.NotNil(t, client)
	defer client.Close()

	var name = "TEST/" + nodeId

	found, err := client.IsNodeExist(ctx, name)
	assert.Nilf(t, err, "get node failed")
	assert.False(t, found)

	err = client.PutKey(ctx, name, "some value", 0)
	assert.Nilf(t, err, "put key failed")

	found, err = client.IsNodeExist(ctx, name)
	assert.Nilf(t, err, "get node failed")
	assert.True(t, found)

	err = client.DelKey(ctx, name)
	assert.Nilf(t, err, "delete node failed")

	found, err = client.IsNodeExist(ctx, name)
	assert.Nilf(t, err, "get node failed")
	assert.False(t, found)
}

func TestEtcdClient_PutNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var client = connectClient(t, ctx)
	require.NotNil(t, client)
	defer client.Close()

	var node = NewNodeFrom(nodeId, 1)
	var name = "TEST/" + nodeId

	err := client.PutNode(ctx, name, node, 0)
	assert.Nilf(t, err, "put node fail")

	info, err := client.GetNode(ctx, name)
	assert.Nilf(t, err, "get node fail")
	assert.Greaterf(t, len(info), 0, "get node empty")

	err = client.DelKey(ctx, name)
	assert.Nilf(t, err, "delete node fail")
}

func TestEtcdClient_ListNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	var client = connectClient(t, ctx)
	require.NotNil(t, client)
	defer client.Close()

	var dir = "TEST_SERVICE"

	for i := 1; i <= 5; i++ {
		var key = fmt.Sprintf("%s/node%d", dir, i)
		var node = NewNodeFrom(strconv.Itoa(i), uint32(i))
		err := client.PutNode(ctx, key, node, 0)
		assert.Nilf(t, err, "put node fail")
	}

	nodes, rev, err := client.ListNodes(ctx, dir)
	assert.Nilf(t, err, "list dir fail")
	assert.Equal(t, len(nodes), 5)
	assert.Greater(t, rev, int64(0))

	_, err = client.DelDir(ctx, dir)
	assert.Nilf(t, err, "del dir fail")

	nodes, rev, err = client.ListNodes(ctx, dir)
	assert.Nilf(t, err, "list dir fail")
	assert.Equal(t, len(nodes), 0)
}

func TestEtcdClient_RegisterNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	var client = connectClient(t, ctx)
	require.NotNil(t, client)
	defer client.Close()

	var node = NewNodeFrom(nodeId, 1)
	var name = "bingo/" + nodeId
	var keeper = NewNodeAliveKeeper(name, node, 5)
	var registerMe = func() {
		t.Logf("try to register %s", name)
		err := keeper.RegisterNode(ctx, client)
		assert.Nilf(t, err, "register node fail")
		assert.Nilf(t, err, "node keep alive fail")
		keeper.alive.Store(true)
	}

	registerMe()
	defer client.DelKey(context.Background(), name)

	var ticker = time.NewTicker(time.Second * 3)
	defer ticker.Stop()
	var ticks = 0
	for {
		select {
		case <-ticker.C:
			ticks++
			fmt.Printf("ticks %d\n", ticks)
			if !keeper.alive.Load() || keeper.leaseId == 0 {
				fmt.Printf("re-register worker at tick %d, in case of etcd server lost\n", ticks)
				client.DelKey(ctx, name)
				registerMe()
			}

		case ka, ok := <-keeper.kaChan:
			if ka != nil {
				fmt.Printf("lease=%d keep alive ttl=%d at rev=%d\n", ka.ID, ka.TTL, ka.Revision)
			} else if !ok || ka == nil {
				fmt.Printf("lease %d is dead, try re-register later\n", keeper.leaseId)
				keeper.alive.Store(false)
				keeper.kaChan = nil
			}

		case <-ctx.Done():
			return
		}
	}
}

func TestEtcdClient_RegisterAndKeepAlive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	var client = connectClient(t, ctx)
	require.NotNil(t, client)
	defer client.Close()

	var node = NewNodeFrom(nodeId, 1)
	var name = "bingo/" + nodeId
	t.Logf("register and keepalive forever, only for 15s")
	keeper, err := client.RegisterAndKeepAlive(ctx, name, node, 5)
	assert.Nilf(t, err, "register and keepalive fail")
	defer client.DelKey(context.Background(), name)

	// wait until timed-out
	<-ctx.Done()
	keeper.Revoke(context.Background(), client)
	t.Logf("done")
}

func TestEtcdClient_WatchDir(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	var client = connectClient(t, ctx)
	require.NotNil(t, client)
	defer client.Close()

	var dir = "/"
	var eventChan = client.WatchDir(ctx, dir, 0)

	var ticker = time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	var tick = 0

	var modKey = func() {
		var id = rand.Int() % tick
		if id == 0 {
			id += 1
		}
		var key = fmt.Sprintf("%s/node/%d", dir, id)
		var node = NewNodeFrom(strconv.Itoa(id), 1)
		if err := client.PutNode(ctx, key, node, 0); err != nil {
			t.Fatalf("set node: %v\n", err)
		}
		if tick > 0 && tick%5 == 0 {
			if err := client.DelKey(ctx, key); err != nil {
				t.Fatalf("del node: %v\n", err)
			}
		}
	}

	for {
		select {
		case <-ticker.C:
			tick++
			modKey()

		case event, ok := <-eventChan:
			if !ok {
				return
			}
			fmt.Printf("event: %v, %s %d, node: %v\n", event.EvType, event.NodeType, event.NodeID, event.Info)

		case <-ctx.Done():
			return
		}
	}
}

func TestEtcdClient_WatchDirTo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var client = connectClient(t, ctx)
	require.NotNil(t, client)
	defer client.Close()

	var nodeMap = NewNodeMap()
	var dir = "/"

	client.WatchDirTo(ctx, dir, []string{"node"}, nodeMap)

	var showNodeMap = func() {
		fmt.Printf("now we have %d nodes\n", nodeMap.TotalCount())
		for _, name := range nodeMap.GetTypeList() {
			var nn = nodeMap.GetNodes(name)
			for _, node := range nn {
				fmt.Printf("  %v\n", node)
			}
		}
	}

	showNodeMap()

	var ticker = time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	var tick = 0

	var modKey = func() {
		var id = rand.Int() % tick
		if id == 0 {
			id += 1
		}
		var key = fmt.Sprintf("%s/node/%d", dir, id)
		var node = NewNodeFrom(strconv.Itoa(id), 1)
		if err := client.PutNode(ctx, key, node, 0); err != nil {
			t.Fatalf("set node: %v\n", err)
		}
		if tick > 0 && tick%5 == 0 {
			if err := client.DelKey(ctx, key); err != nil {
				t.Fatalf("del node: %v\n", err)
			}
		}
	}

	for {
		select {
		case <-ticker.C:
			tick++
			modKey()

		case <-ctx.Done():
			showNodeMap()
			return
		}
	}
}
