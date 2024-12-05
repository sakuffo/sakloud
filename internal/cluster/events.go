package cluster

import "time"

// EventType defines the type of cluster event
type EventType int

const (
	NodeJoin EventType = iota
	NodeLeave
	NodeStateChange
)

// ClusterEvent represents a change in the cluster state
type ClusterEvent struct {
	Type      EventType
	Node      *Node
	Timestamp time.Time
}

// ClusterEventFunc is a callback function type for cluster events
type ClusterEventFunc func(event ClusterEvent) 