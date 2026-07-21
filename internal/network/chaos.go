package network

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ChaosConfig specifies network fault injection parameters
type ChaosConfig struct {
	LatencyMillis int     // Artificial latency added to connections
	PacketDropRate float64 // Probability (0.0 to 1.0) of dropping a packet/connection
}

// ChaosNetwork manages simulated network partitions and fault injection
type ChaosNetwork struct {
	mu               sync.RWMutex
	partitionedNodes map[string]bool
	config           ChaosConfig
}

// NewChaosNetwork initializes a network chaos controller
func NewChaosNetwork(cfg ChaosConfig) *ChaosNetwork {
	return &ChaosNetwork{
		partitionedNodes: make(map[string]bool),
		config:           cfg,
	}
}

// IsolateNode isolates a node by placing it into a network partition
func (cn *ChaosNetwork) IsolateNode(nodeID string) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	cn.partitionedNodes[nodeID] = true
}

// HealPartition removes all network partitions and restores normal connectivity
func (cn *ChaosNetwork) HealPartition() {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	cn.partitionedNodes = make(map[string]bool)
}

// HealNode removes a specific node from partition
func (cn *ChaosNetwork) HealNode(nodeID string) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	delete(cn.partitionedNodes, nodeID)
}

// IsPartitioned checks if a node is currently partitioned off
func (cn *ChaosNetwork) IsPartitioned(nodeID string) bool {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	return cn.partitionedNodes[nodeID]
}

// ShouldDropConnection evaluates whether a connection attempt should be dropped under chaos rules
func (cn *ChaosNetwork) ShouldDropConnection(fromNodeID, toNodeID string) bool {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	if cn.partitionedNodes[fromNodeID] || cn.partitionedNodes[toNodeID] {
		return true // Drop connection due to network partition
	}

	if cn.config.PacketDropRate > 0 && rand.Float64() < cn.config.PacketDropRate {
		return true // Random packet drop injection
	}

	return false
}

// ApplyLatency sleeps to simulate network delay if configured
func (cn *ChaosNetwork) ApplyLatency() {
	if cn.config.LatencyMillis > 0 {
		time.Sleep(time.Duration(cn.config.LatencyMillis) * time.Millisecond)
	}
}

// Status returns a human-readable summary of current chaos state
func (cn *ChaosNetwork) Status() string {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	return fmt.Sprintf("Partitioned Nodes: %d, Latency: %dms, Drop Rate: %.2f", len(cn.partitionedNodes), cn.config.LatencyMillis, cn.config.PacketDropRate)
}
