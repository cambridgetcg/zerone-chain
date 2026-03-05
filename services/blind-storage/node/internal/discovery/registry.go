package discovery

import (
	"sync"
	"time"
)

// Node represents a registered storage node.
type Node struct {
	Address      string
	Endpoint     string // gRPC/HTTP endpoint
	StakeAmount  int64
	CapacityGB   int64
	AssignedGB   int64
	Active       bool
	RegisteredAt time.Time
	LastSeen     time.Time
}

// Registry tracks active storage nodes (in-memory for now).
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]*Node // address → node
}

// NewRegistry creates a node registry.
func NewRegistry() *Registry {
	return &Registry{nodes: make(map[string]*Node)}
}

// Register adds or updates a node.
func (r *Registry) Register(node *Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	node.Active = true
	node.LastSeen = time.Now()
	if existing, ok := r.nodes[node.Address]; ok {
		existing.Endpoint = node.Endpoint
		existing.CapacityGB = node.CapacityGB
		existing.StakeAmount = node.StakeAmount
		existing.Active = true
		existing.LastSeen = time.Now()
	} else {
		node.RegisteredAt = time.Now()
		r.nodes[node.Address] = node
	}
}

// Deregister marks a node as inactive.
func (r *Registry) Deregister(address string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n, ok := r.nodes[address]; ok {
		n.Active = false
	}
}

// GetNode returns a node by address.
func (r *Registry) GetNode(address string) (*Node, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.nodes[address]
	return n, ok
}

// ActiveNodes returns all active nodes.
func (r *Registry) ActiveNodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*Node
	for _, n := range r.nodes {
		if n.Active {
			result = append(result, n)
		}
	}
	return result
}

// NodesWithCapacity returns active nodes with available capacity.
func (r *Registry) NodesWithCapacity(minFreeGB int64) []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*Node
	for _, n := range r.nodes {
		if n.Active && (n.CapacityGB-n.AssignedGB) >= minFreeGB {
			result = append(result, n)
		}
	}
	return result
}

// Heartbeat updates a node's last seen timestamp.
func (r *Registry) Heartbeat(address string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.nodes[address]
	if ok {
		n.LastSeen = time.Now()
	}
	return ok
}

// PruneStale marks nodes as inactive if they haven't been seen in the given duration.
func (r *Registry) PruneStale(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	pruned := 0
	for _, n := range r.nodes {
		if n.Active && n.LastSeen.Before(cutoff) {
			n.Active = false
			pruned++
		}
	}
	return pruned
}
