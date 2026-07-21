package network

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTLSConfigManagerAndmTLSHandshake(t *testing.T) {
	mgr, err := NewTLSConfigManager()
	assert.NoError(t, err)
	assert.NotNil(t, mgr)

	node1Cert, err := mgr.GenerateNodeKeyPair("3000")
	assert.NoError(t, err)

	node2Cert, err := mgr.GenerateNodeKeyPair("3001")
	assert.NoError(t, err)

	serverTLS := mgr.GetServerTLSConfig(node1Cert)
	clientTLS := mgr.GetClientTLSConfig(node2Cert)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Skipf("skipping test due to network bind restriction: %v", err)
		return
	}
	defer listener.Close()

	addr := listener.Addr().String()

	serverErrCh := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErrCh <- err
			return
		}
		defer conn.Close()
		buf := make([]byte, 10)
		_, err = conn.Read(buf)
		serverErrCh <- err
	}()

	conn, err := tls.Dial("tcp", addr, clientTLS)
	assert.NoError(t, err)
	if err == nil {
		_, _ = conn.Write([]byte("ping"))
		_ = conn.Close()
	}

	select {
	case err := <-serverErrCh:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("mTLS handshake timed out")
	}
}

func TestChaosNetworkPartitionAndHealing(t *testing.T) {
	chaos := NewChaosNetwork(ChaosConfig{
		LatencyMillis: 10,
		PacketDropRate: 0.0,
	})

	assert.False(t, chaos.IsPartitioned("node-1"))
	assert.False(t, chaos.ShouldDropConnection("node-1", "node-2"))

	// Isolate node-1
	chaos.IsolateNode("node-1")
	assert.True(t, chaos.IsPartitioned("node-1"))
	assert.True(t, chaos.ShouldDropConnection("node-1", "node-2"))
	assert.True(t, chaos.ShouldDropConnection("node-3", "node-1"))
	assert.False(t, chaos.ShouldDropConnection("node-2", "node-3"))

	// Heal node-1
	chaos.HealNode("node-1")
	assert.False(t, chaos.IsPartitioned("node-1"))
	assert.False(t, chaos.ShouldDropConnection("node-1", "node-2"))

	// Heal entire partition
	chaos.IsolateNode("node-2")
	chaos.HealPartition()
	assert.False(t, chaos.IsPartitioned("node-2"))
}

func TestUnauthenticatedTLSClientRejected(t *testing.T) {
	mgr, err := NewTLSConfigManager()
	assert.NoError(t, err)

	node1Cert, err := mgr.GenerateNodeKeyPair("3000")
	assert.NoError(t, err)

	serverTLS := mgr.GetServerTLSConfig(node1Cert)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Skipf("skipping test due to network bind restriction: %v", err)
		return
	}
	defer listener.Close()

	addr := listener.Addr().String()

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	// Connect with plain unauthenticated client config (no client certificate)
	unauthClientTLS := &tls.Config{InsecureSkipVerify: true}
	conn, err := tls.Dial("tcp", addr, unauthClientTLS)
	if err == nil {
		_, writeErr := conn.Write([]byte("unauthenticated"))
		_ = conn.Close()
		assert.Error(t, writeErr, "Server must reject unauthenticated client without mTLS cert")
	}
}
