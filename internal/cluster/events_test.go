package cluster

import (
	"net"
	"testing"
	"time"
)

func TestClusterEvents(t *testing.T) {
	// Create a test node
	node := NewNode("test", net.ParseIP("192.168.1.1"), 7946)
	
	// Create test events
	events := []struct {
		eventType EventType
		expected  string
	}{
		{NodeJoin, "join"},
		{NodeLeave, "leave"},
		{NodeStateChange, "state-change"},
	}

	// Test each event
	for _, e := range events {
		event := ClusterEvent{
			Type:      e.eventType,
			Node:      node,
			Timestamp: time.Now(),
		}

		// Verify event properties
		if event.Type != e.eventType {
			t.Errorf("Expected event type %v, got %v", e.eventType, event.Type)
		}
		if event.Node != node {
			t.Errorf("Expected node reference to match for event type %v", e.eventType)
		}
		if event.Timestamp.IsZero() {
			t.Errorf("Expected non-zero timestamp for event type %v", e.eventType)
		}
	}
} 