package cluster

import (
	"time"
)

type HealthCheck struct {
	Timestamp time.Time
	Status    NodeState
	Message   string
}

func (c *Cluster) StartHealthCheck() {
	c.logger.Info("Starting health check service with %v interval", c.config.HealthInterval)
	c.healthCheck = time.NewTicker(c.config.HealthInterval)

	// Start heartbeat goroutine
	go c.runHeartbeat()

	go func() {
		for {
			select {
			case <-c.ctx.Done():
				c.logger.Info("Health check service stopping due to context cancellation")
				return
			case <-c.healthCheck.C:
				c.checkNodes()
			}
		}
	}()
}

// runHeartbeat periodically updates the local node's LastSeen time
func (c *Cluster) runHeartbeat() {
	heartbeatTicker := time.NewTicker(c.config.HealthInterval / 2) // More frequent than health check
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-heartbeatTicker.C:
			c.mutex.Lock()
			if localNode, exists := c.nodes[c.localNode.ID]; exists {
				localNode.LastSeen = time.Now()
				c.logger.Debug("Updated local node heartbeat")
			}
			c.mutex.Unlock()
		}
	}
}

func (c *Cluster) checkNodes() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	nodeCount := len(c.nodes)

	// Log detailed node states only when there are changes or every minute
	shouldLogDetails := false
	if c.lastDetailedLog.IsZero() || now.Sub(c.lastDetailedLog) >= time.Minute {
		shouldLogDetails = true
		c.lastDetailedLog = now
	}

	if shouldLogDetails {
		c.logger.Debug("Current cluster state - %d nodes (local node ID: %s)", nodeCount, c.localNode.ID)
		for id, node := range c.nodes {
			lastSeenAgo := now.Sub(node.LastSeen).Round(time.Second)
			nodeType := "remote"
			if id == c.localNode.ID {
				nodeType = "local"
			}
			c.logger.Debug("%s node state - ID: %s, Name: %s, Address: %s, State: %v, LastSeen: %v ago",
				nodeType, id, node.Name, node.Address, node.State, lastSeenAgo)
		}
	} else {
		c.logger.Debug("Health check cycle - %d nodes", nodeCount)
	}

	for _, node := range c.nodes {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.checkNodeHealth(node, now)
		}
	}
}

func (c *Cluster) checkNodeHealth(node *Node, now time.Time) {
	lastSeenDuration := now.Sub(node.LastSeen)
	isLocal := node.ID == c.localNode.ID
	nodeType := "remote"
	if isLocal {
		nodeType = "local"
	}
	
	// Log health check details when approaching timeouts
	if lastSeenDuration >= c.config.SuspectTimeout/2 {
		c.logger.Debug("Checking %s node health - Name: %s, ID: %s, State: %v, LastSeen: %v ago",
			nodeType, node.Name, node.ID, node.State, lastSeenDuration.Round(time.Second))
	}

	if lastSeenDuration > c.config.SuspectTimeout && node.State == Alive {
		c.logger.Warn("%s node %s (%s) hasn't been seen for %v (suspect timeout: %v), marking as suspect",
			nodeType, node.Name, node.Address, lastSeenDuration.Round(time.Second), c.config.SuspectTimeout)
		c.markNodeSuspect(node)
	}

	if lastSeenDuration > c.config.DeadTimeout && node.State != Dead {
		c.logger.Warn("%s node %s (%s) hasn't been seen for %v (dead timeout: %v), marking as dead",
			nodeType, node.Name, node.Address, lastSeenDuration.Round(time.Second), c.config.DeadTimeout)
		c.markNodeDead(node)
	}
}

func (c *Cluster) markNodeSuspect(node *Node) {
	if node.State == Alive {
		oldState := node.State
		node.State = Suspect
		c.logger.Info("Node %s (%s) state changed from %v to %v",
			node.Name, node.Address, oldState, node.State)
		c.notifySubscribers(ClusterEvent{
			Type:      NodeStateChange,
			Node:      node,
			Timestamp: time.Now(),
		})
	}
}

func (c *Cluster) markNodeDead(node *Node) {
	if node.State != Dead {
		oldState := node.State
		node.State = Dead
		c.logger.Info("Node %s (%s) state changed from %v to %v",
			node.Name, node.Address, oldState, node.State)
		c.notifySubscribers(ClusterEvent{
			Type:      NodeLeave,
			Node:      node,
			Timestamp: time.Now(),
		})

		go c.removeDeadNodeAfterDelay(node.ID)
	}
}

func (c *Cluster) removeDeadNodeAfterDelay(nodeID string) {
	c.logger.Debug("Starting %v delay before removing dead node: %s", c.config.NodeRemovalDelay, nodeID)
	
	select {
	case <-c.ctx.Done():
		c.logger.Debug("Cancelling node removal due to context cancellation: %s", nodeID)
		return
	case <-time.After(c.config.NodeRemovalDelay):
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if node, exists := c.nodes[nodeID]; exists && node.State == Dead {
		c.logger.Info("Removing dead node %s (%s) from cluster after %v delay",
			node.Name, node.Address, c.config.NodeRemovalDelay)
		delete(c.nodes, nodeID)
	} else {
		c.logger.Debug("Node %s no longer eligible for removal (state changed or already removed)", nodeID)
	}
}
