package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

// StartDiscovery initializes mDNS service and discovery.
// It starts a background goroutine that continuously discovers peers until
// the cluster context is cancelled.
func (c *Cluster) StartDiscovery(serviceName string) error {
	c.logger.Info("Starting mDNS service with name: %s", serviceName)

	// start server for advertising
	server, err := zeroconf.Register(
		c.localNode.Name,
		serviceName,
		"local.",
		c.localNode.Port,
		[]string{fmt.Sprintf("id=%s", c.localNode.ID)},
		nil,
	)
	if err != nil {
		c.logger.Error("Failed to register mDNS service: %v", err)
		return fmt.Errorf("failed to register zeroconf server: %w", err)
	}

	// Store server for cleanup
	c.server = server
	c.logger.Info("mDNS service registered successfully on port %d", c.localNode.Port)

	// start discovery
	c.discoveryWG.Add(1)
	go func() {
		defer c.discoveryWG.Done()
		c.discover(serviceName)
	}()
	return nil
}

// discover continuously looks for other nodes until context is cancelled
func (c *Cluster) discover(serviceName string) {
	c.logger.Info("Starting node discovery for service: %s", serviceName)
	ticker := time.NewTicker(c.config.DiscoveryRetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Discovery service stopping due to context cancellation")
			return
		case <-ticker.C:
			c.logger.Debug("Starting new discovery cycle (service: %s, retry interval: %v)", 
				serviceName, c.config.DiscoveryRetryInterval)
			if err := c.runDiscoveryCycle(serviceName); err != nil {
				c.logger.Error("Discovery cycle failed: %v", err)
			}
		}
	}
}

// runDiscoveryCycle performs a single discovery cycle
func (c *Cluster) runDiscoveryCycle(serviceName string) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("failed to create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(c.ctx, c.config.DiscoveryTimeout)
	defer cancel()

	go func() {
		for entry := range entries {
			c.handleDiscoveredNode(entry)
		}
	}()

	if err := resolver.Browse(ctx, serviceName, "local.", entries); err != nil {
		return fmt.Errorf("failed to browse services: %w", err)
	}

	return nil
}

func (c *Cluster) handleDiscoveredNode(entry *zeroconf.ServiceEntry) {
	// Extract node ID from TXT records
	var nodeID string
	metadata := make(map[string]string)
	
	// Process all TXT records
	for _, txt := range entry.Text {
		if parts := strings.SplitN(txt, "=", 2); len(parts) == 2 {
			if parts[0] == "id" {
				nodeID = parts[1]
			}
			metadata[parts[0]] = parts[1]
		}
	}

	if nodeID == "" {
		c.logger.Debug("Discovered node missing ID in TXT records - Host: %s, IP: %v, Port: %d",
			entry.HostName, entry.AddrIPv4, entry.Port)
		return
	}

	if nodeID == c.localNode.ID {
		c.logger.Debug("Skipping discovery of our own node (ID: %s, Host: %s)",
			nodeID, entry.HostName)
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we already know about this node
	if existingNode, exists := c.nodes[nodeID]; exists {
		c.logger.Debug("Updating existing node - ID: %s, Name: %s, Address: %s, Last State: %v",
			nodeID, existingNode.Name, existingNode.Address, existingNode.State)
		existingNode.LastSeen = time.Now()
		if existingNode.State == Suspect {
			c.logger.Info("Node %s recovered from suspect state", existingNode.Name)
			existingNode.State = Alive
			c.notifySubscribers(ClusterEvent{
				Type:      NodeStateChange,
				Node:      existingNode,
				Timestamp: time.Now(),
			})
		}
	} else {
		// Create new node
		newNode := NewNode(entry.HostName, entry.AddrIPv4[0], entry.Port)
		newNode.ID = nodeID
		newNode.Metadata = metadata
		c.nodes[nodeID] = newNode
		c.logger.Debug("Added new node to cluster - ID: %s, Name: %s, Address: %s",
			newNode.ID, newNode.Name, newNode.Address)
		c.notifySubscribers(ClusterEvent{
			Type:      NodeJoin,
			Node:      newNode,
			Timestamp: time.Now(),
		})
	}
}
