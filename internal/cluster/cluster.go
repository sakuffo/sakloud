package cluster

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/sakuffo/sakloud/internal/config"
	"github.com/sakuffo/sakloud/internal/logger"
)

// Cluster manages the state of all nodes in the cluster. It handles node discovery,
// health checking, and event distribution to subscribers.
type Cluster struct {
	ctx         context.Context
	cancel      context.CancelFunc
	mutex       sync.RWMutex
	localNode   *Node
	nodes       map[string]*Node
	subscribers []ClusterEventFunc
	server      *zeroconf.Server
	logger      logger.Logger
	config      *config.Config
	healthCheck *time.Ticker
	discoveryWG sync.WaitGroup
	lastDetailedLog time.Time  // Tracks when we last logged detailed node states
}

// NewCluster initializes a new cluster with the local node.
//
// The provided context controls the lifecycle of all cluster operations including
// discovery and health checking. Cancel the context to initiate a graceful shutdown.
//
// Example:
//
//	ctx := context.Background()
//	cfg := config.DefaultConfig()
//	logger := logger.New("cluster")
//	localNode := cluster.NewNode("local", net.ParseIP("192.168.1.1"), 7946)
//
//	c := cluster.NewCluster(ctx, localNode, logger, cfg)
//	defer c.Shutdown()
func NewCluster(ctx context.Context, localNode *Node, log logger.Logger, cfg *config.Config) *Cluster {
	ctx, cancel := context.WithCancel(ctx)
	c := &Cluster{
		ctx:       ctx,
		cancel:    cancel,
		localNode: localNode,
		nodes:     make(map[string]*Node),
		logger:    log,
		config:    cfg,
	}

	// Add local node to the nodes map
	c.nodes[localNode.ID] = localNode

	c.logger.Info("Created new cluster with local node: %s (ID: %s, Address: %s:%d)",
		localNode.Name, localNode.ID, localNode.Address, localNode.Port)
	return c
}

// AddNode adds a node to the cluster or updates an existing one.
// This method is thread-safe.
func (c *Cluster) AddNode(node *Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Skip if trying to add our own node
	if node.ID == c.localNode.ID {
		c.logger.Debug("Skipping add/update of our own node ID: %s", node.ID)
		return
	}

	if existing, exists := c.nodes[node.ID]; exists {
		c.updateNodeInfo(existing, node.Address, node.Port)
		return
	}

	c.nodes[node.ID] = node
	c.logger.Info("Added new node to cluster: %s (ID: %s, Address: %s:%d)",
		node.Name, node.ID, node.Address, node.Port)
	c.notifySubscribers(ClusterEvent{
		Type:      NodeJoin,
		Node:      node,
		Timestamp: time.Now(),
	})
}

// updateNodeInfo updates a node's network information and state.
// Caller must hold the cluster mutex.
func (c *Cluster) updateNodeInfo(node *Node, newAddress net.IP, newPort int) {
	// Update last seen time
	oldLastSeen := node.LastSeen
	node.LastSeen = time.Now()
	
	c.logger.Debug("Updating node %s (ID: %s) - Last seen delta: %v",
		node.Name, node.ID, node.LastSeen.Sub(oldLastSeen))
	
	// Check and update network info
	if !node.Address.Equal(newAddress) {
		c.logger.Info("Node %s (ID: %s) address changed from %s to %s",
			node.Name, node.ID, node.Address, newAddress)
		node.Address = newAddress
	}
	
	if node.Port != newPort {
		c.logger.Info("Node %s (ID: %s) port changed from %d to %d",
			node.Name, node.ID, node.Port, newPort)
		node.Port = newPort
	}

	// Check if node needs to be revived
	if node.State != Alive {
		c.reviveNode(node)
	}
}

// reviveNode marks a previously suspect or dead node as alive
func (c *Cluster) reviveNode(node *Node) {
	oldState := node.State
	node.State = Alive
	c.logger.Info("Node %s (ID: %s) state changed from %v to %v",
		node.Name, node.ID, oldState, Alive)
	c.notifySubscribers(ClusterEvent{
		Type:      NodeStateChange,
		Node:      node,
		Timestamp: time.Now(),
	})
}

// Subscribe adds an event listener that will be notified of cluster events.
// Events are delivered asynchronously to all subscribers.
func (c *Cluster) Subscribe(fn ClusterEventFunc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.subscribers = append(c.subscribers, fn)
	c.logger.Debug("Added new cluster event subscriber (total subscribers: %d)",
		len(c.subscribers))
}

// notifySubscribers sends events to all subscribers asynchronously
func (c *Cluster) notifySubscribers(event ClusterEvent) {
	c.logger.Debug("Notifying %d subscribers of event type: %v",
		len(c.subscribers), event.Type)
	for _, fn := range c.subscribers {
		go fn(event)
	}
}

// Shutdown gracefully shuts down the cluster, stopping all background services
// and notifying peers of departure. This method blocks until shutdown is complete
// or the context deadline is exceeded.
func (c *Cluster) Shutdown() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.logger.Info("Starting cluster shutdown for node: %s", c.localNode.Name)

	// Cancel context to stop all background operations
	c.cancel()

	// Clean up zeroconf(mDNS) server if it exists
	if c.server != nil {
		c.logger.Debug("Shutting down mDNS server for node: %s", c.localNode.Name)
		c.server.Shutdown()
	}

	// Stop health check ticker if running
	if c.healthCheck != nil {
		c.healthCheck.Stop()
	}

	// Wait for discovery goroutines to finish
	c.discoveryWG.Wait()

	// Notify subscribers about shutdown
	c.logger.Info("Notifying %d subscribers of node departure", len(c.subscribers))
	c.notifySubscribers(ClusterEvent{
		Type:      NodeLeave,
		Node:      c.localNode,
		Timestamp: time.Now(),
	})

	c.logger.Info("Cluster shutdown complete for node: %s", c.localNode.Name)
	return nil
}

// GetNode returns a node by its ID if it exists in the cluster
func (c *Cluster) GetNode(id string) (*Node, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	node, exists := c.nodes[id]
	return node, exists
}

// IsLocalNode checks if the given node ID matches the local node
func (c *Cluster) IsLocalNode(id string) bool {
	return id == c.localNode.ID
}
