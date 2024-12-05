package cluster

import (
	"net"
	"testing"
)

func TestNewNode(t *testing.T) {
	name := "test-node"
	ip := net.ParseIP("192.168.1.1")
	port := 7946

	node := NewNode(name, ip, port)

	// Test basic properties
	if node.Name != name {
		t.Errorf("Expected name %s, got %s", name, node.Name)
	}
	if !node.Address.Equal(ip) {
		t.Errorf("Expected IP %v, got %v", ip, node.Address)
	}
	if node.Port != port {
		t.Errorf("Expected port %d, got %d", port, node.Port)
	}

	// Test default values
	if node.State != Alive {
		t.Errorf("Expected initial state to be Alive, got %v", node.State)
	}
	if node.ID == "" {
		t.Error("Expected non-empty UUID")
	}
	if node.Metadata == nil {
		t.Error("Expected initialized metadata map")
	}
} 