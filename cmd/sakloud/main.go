package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sakuffo/sakloud/internal/cluster"
	"github.com/sakuffo/sakloud/internal/config"
	"github.com/sakuffo/sakloud/internal/logger"
)

var (
	logLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error, fatal)")
)

func parseLogLevel(level string) logger.Level {
	switch strings.ToLower(level) {
	case "debug":
		return logger.DebugLevel
	case "warn":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	case "fatal":
		return logger.FatalLevel
	default:
		return logger.InfoLevel
	}
}

// isLinkLocal checks if an IP is a link-local address (169.254.x.x)
func isLinkLocal(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 169 && ip4[1] == 254
	}
	return false
}

// isLoopback checks if an IP is a loopback address (127.x.x.x)
func isLoopback(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 127
	}
	return false
}

// isPrivateNetwork checks if an IP is in private network ranges
func isPrivateNetwork(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		// Check for private network ranges:
		// 10.0.0.0/8
		// 172.16.0.0/12
		// 192.168.0.0/16
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}
	return false
}

func main() {
	flag.Parse()

	// Initialize logger
	log, err := logger.New("sakloud")
	if err != nil {
		panic(err)
	}
	defer log.Close()

	// Set log level from command line flag
	log.SetLevel(parseLogLevel(*logLevel))

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

	localIP := cluster.SelectBestIP(addrs)
	if localIP == nil {
		log.Fatal("Could not find suitable local IP address (non-link-local, non-loopback)")
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
