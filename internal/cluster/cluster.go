package cluster

import (
	"context"
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
	c.logger.Info("Created new cluster with local node: %s (ID: %s, Address: %s:%d)",
		localNode.Name, localNode.ID, localNode.Address, localNode.Port)
	return c
}

// AddNode adds a node to the cluster or updates its last seen time if it already exists.
// This method is thread-safe.
func (c *Cluster) AddNode(node *Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if existing, exists := c.nodes[node.ID]; exists {
		existing.LastSeen = time.Now()
		c.logger.Debug("Updated last seen time for node: %s (ID: %s, Address: %s:%d)",
			node.Name, node.ID, node.Address, node.Port)
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
