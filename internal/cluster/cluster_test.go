package cluster

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sakuffo/sakloud/internal/config"
	"github.com/sakuffo/sakloud/internal/logger"
)

type mockLogger struct {
	logger.Logger
}

func (m *mockLogger) Info(format string, v ...interface{})  {}
func (m *mockLogger) Debug(format string, v ...interface{}) {}
func (m *mockLogger) Error(format string, v ...interface{}) {}
func (m *mockLogger) Warn(format string, v ...interface{})  {}
func (m *mockLogger) Fatal(format string, v ...interface{}) {}

func TestNewCluster(t *testing.T) {
	// Setup
	ctx := context.Background()
	cfg := config.DefaultConfig()
	log := &mockLogger{}
	localNode := NewNode("test-node", net.ParseIP("127.0.0.1"), 7946)

	// Create cluster
	c := NewCluster(ctx, localNode, log, cfg)
	if c == nil {
		t.Fatal("Expected non-nil cluster")
	}

	// Verify initial state
	if c.localNode != localNode {
		t.Error("Local node not set correctly")
	}
	if len(c.nodes) != 0 {
		t.Error("Expected empty nodes map")
	}
}

func TestAddNode(t *testing.T) {
	// Setup cluster
	ctx := context.Background()
	cfg := config.DefaultConfig()
	log := &mockLogger{}
	localNode := NewNode("local", net.ParseIP("127.0.0.1"), 7946)
	c := NewCluster(ctx, localNode, log, cfg)

	// Create test node
	testNode := NewNode("test", net.ParseIP("127.0.0.2"), 7946)

	// Test adding node
	c.AddNode(testNode)
	if len(c.nodes) != 1 {
		t.Error("Expected one node in cluster")
	}

	// Test node exists
	if node, exists := c.nodes[testNode.ID]; !exists {
		t.Error("Node not found in cluster")
	} else if node.Name != "test" {
		t.Error("Node name mismatch")
	}
}

func TestClusterShutdown(t *testing.T) {
	// Setup
	ctx := context.Background()
	cfg := config.DefaultConfig()
	log := &mockLogger{}
	localNode := NewNode("test-node", net.ParseIP("127.0.0.1"), 7946)
	c := NewCluster(ctx, localNode, log, cfg)

	// Start discovery and health check
	if err := c.StartDiscovery(cfg.ServiceName); err != nil {
		t.Fatalf("Failed to start discovery: %v", err)
	}
	c.StartHealthCheck()

	// Test shutdown
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		if err := c.Shutdown(); err != nil {
			t.Errorf("Shutdown error: %v", err)
		}
	}()

	// Wait for shutdown with timeout
	select {
	case <-shutdownDone:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Shutdown timed out")
	}
}

func TestNodeStateTransitions(t *testing.T) {
	// Setup
	ctx := context.Background()
	cfg := config.DefaultConfig()
	// Use shorter timeouts for testing, but not too short
	cfg.SuspectTimeout = 300 * time.Millisecond
	cfg.DeadTimeout = 900 * time.Millisecond
	cfg.HealthInterval = 100 * time.Millisecond

	log := &mockLogger{}
	localNode := NewNode("local", net.ParseIP("127.0.0.1"), 7946)
	c := NewCluster(ctx, localNode, log, cfg)

	// Add a test node that was last seen recently
	testNode := NewNode("test", net.ParseIP("127.0.0.2"), 7946)
	testNode.LastSeen = time.Now() // Start with a fresh timestamp
	c.AddNode(testNode)

	// Start health checking
	c.StartHealthCheck()

	// Helper function to safely get node state
	getNodeState := func() NodeState {
		c.mutex.RLock()
		defer c.mutex.RUnlock()
		if node, exists := c.nodes[testNode.ID]; exists {
			return node.State
		}
		return Unknown
	}

	// Verify initial state is Alive
	if state := getNodeState(); state != Alive {
		t.Errorf("Initial state should be Alive, got %v", state)
	}

	// Update LastSeen to trigger Suspect state
	c.mutex.Lock()
	if node, exists := c.nodes[testNode.ID]; exists {
		node.LastSeen = time.Now().Add(-400 * time.Millisecond)
	}
	c.mutex.Unlock()

	// Wait and verify transition to Suspect
	time.Sleep(400 * time.Millisecond)
	if state := getNodeState(); state != Suspect {
		t.Errorf("Expected node state to be Suspect, got %v", state)
	}

	// Wait and verify transition to Dead
	time.Sleep(600 * time.Millisecond)
	if state := getNodeState(); state != Dead {
		t.Errorf("Expected node state to be Dead, got %v", state)
	}

	// Clean up
	if err := c.Shutdown(); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
} 