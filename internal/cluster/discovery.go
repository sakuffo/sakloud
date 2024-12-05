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
			c.logger.Debug("Starting new discovery cycle")
			if err := c.runDiscoveryCycle(serviceName); err != nil {
				c.logger.Error("Discovery cycle failed: %v", err)
			}
		}
	}
}

func (c *Cluster) runDiscoveryCycle(serviceName string) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("failed to create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, c.config.DiscoveryBufferSize)
	ctx, cancel := context.WithTimeout(c.ctx, c.config.DiscoveryTimeout)
	defer cancel()

	go func(results <-chan *zeroconf.ServiceEntry) {
		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-results:
				if !ok {
					return
				}
				c.handleDiscoveredNode(entry)
			}
		}
	}(entries)

	if err := resolver.Browse(ctx, serviceName, "local.", entries); err != nil {
		return fmt.Errorf("failed to browse services: %w", err)
	}

	return nil
}

func (c *Cluster) handleDiscoveredNode(entry *zeroconf.ServiceEntry) {
	// Skip our own node
	if entry.AddrIPv4[0].String() == c.localNode.Address.String() {
		c.logger.Debug("Skipping own node entry: %s", entry.Instance)
		return
	}

	var nodeID string
	for _, txt := range entry.Text {
		if strings.HasPrefix(txt, "id=") {
			nodeID = strings.TrimPrefix(txt, "id=")
			break
		}
	}

	if nodeID == "" {
		c.logger.Warn("Received entry without node ID: %s", entry.Instance)
		return
	}

	node := &Node{
		ID:       nodeID,
		Name:     entry.Instance,
		Address:  entry.AddrIPv4[0],
		Port:     entry.Port,
		State:    Alive,
		LastSeen: time.Now(),
		Metadata: make(map[string]string),
	}

	c.AddNode(node)
	c.logger.Debug("Discovered node: %s (ID: %s, Address: %s:%d)",
		node.Name, node.ID, node.Address, node.Port)
}
