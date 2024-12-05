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

func (c *Cluster) checkNodes() {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.logger.Debug("Starting health check cycle for %d nodes", len(c.nodes))
	now := time.Now()

	for _, node := range c.nodes {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.checkNodeHealth(node, now)
		}
	}
	c.logger.Debug("Health check cycle complete")
}

func (c *Cluster) checkNodeHealth(node *Node, now time.Time) {
	lastSeenDuration := now.Sub(node.LastSeen)
	c.logger.Debug("Checking node %s (ID: %s), last seen %v ago",
		node.Name, node.ID, lastSeenDuration.Round(time.Second))

	if lastSeenDuration > c.config.SuspectTimeout && node.State == Alive {
		c.logger.Warn("Node %s (%s) hasn't been seen for %v, marking as suspect",
			node.Name, node.Address, lastSeenDuration.Round(time.Second))
		c.markNodeSuspect(node)
	}

	if lastSeenDuration > c.config.DeadTimeout && node.State != Dead {
		c.logger.Warn("Node %s (%s) hasn't been seen for %v, marking as dead",
			node.Name, node.Address, lastSeenDuration.Round(time.Second))
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
