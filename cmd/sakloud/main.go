package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sakuffo/sakloud/internal/cluster"
	"github.com/sakuffo/sakloud/internal/config"
	"github.com/sakuffo/sakloud/internal/logger"
)

func main() {
	// Initialize logger
	log, err := logger.New("sakloud")
	if err != nil {
		panic(err)
	}
	defer log.Close()

	log.Info("Starting Sakloud...")

	// Create root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg := config.DefaultConfig()
	log.Debug("Loaded configuration: %+v", cfg)

	// Get local IP address
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal("Failed to get interface addresses: %v", err)
	}

	var localIP net.IP
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				localIP = ipnet.IP
				break
			}
		}
	}

	if localIP == nil {
		log.Fatal("Could not find suitable local IP address")
	}
	log.Info("Using local IP: %s", localIP.String())

	// Create local node
	localNode := cluster.NewNode("local-node", localIP, cfg.DiscoveryPort)
	log.Info("Created local node: %s (ID: %s)", localNode.Name, localNode.ID)

	// Create and start cluster
	c := cluster.NewCluster(ctx, localNode, log, cfg)
	log.Info("Created cluster instance")

	// Subscribe to cluster events
	c.Subscribe(func(event cluster.ClusterEvent) {
		switch event.Type {
		case cluster.NodeJoin:
			log.Info("Node joined: %s (%s)", event.Node.Name, event.Node.Address)
		case cluster.NodeLeave:
			log.Info("Node left: %s (%s)", event.Node.Name, event.Node.Address)
		case cluster.NodeStateChange:
			log.Info("Node state changed: %s (%s) -> %v", event.Node.Name, event.Node.Address, event.Node.State)
		}
	})

	// Start discovery
	log.Info("Starting cluster discovery...")
	if err := c.StartDiscovery(cfg.ServiceName); err != nil {
		log.Fatal("Failed to start discovery: %v", err)
	}
	log.Info("Cluster discovery started successfully")

	// Start health checking
	log.Info("Starting health check...")
	c.StartHealthCheck()
	log.Info("Health check started successfully")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Waiting for interrupt signal...")

	// Wait for either context cancellation or interrupt signal
	select {
	case <-ctx.Done():
		log.Info("Context cancelled, initiating shutdown...")
	case sig := <-sigChan:
		log.Info("Received signal %v, initiating shutdown...", sig)
		cancel() // Cancel context to initiate graceful shutdown
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		if err := c.Shutdown(); err != nil {
			log.Error("Error during shutdown: %v", err)
		}
	}()

	// Wait for shutdown to complete or timeout
	select {
	case <-shutdownCtx.Done():
		log.Error("Shutdown timed out")
	case <-shutdownDone:
		log.Info("Shutdown complete")
	}
}
