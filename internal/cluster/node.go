package cluster

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// NodeState represents the current state of a node in the cluster
type NodeState int

const (
	Unknown NodeState = iota
	Alive
	Suspect
	Dead
)

// Node represents a single node in the cluster
type Node struct {
	ID       string
	Name     string
	Address  net.IP
	Port     int
	State    NodeState
	LastSeen time.Time
	Metadata map[string]string
}

// NewNode creates a new node instance
func NewNode(name string, address net.IP, port int) *Node {
	return &Node{
		ID:       uuid.New().String(),
		Name:     name,
		Address:  address,
		Port:     port,
		State:    Alive,
		LastSeen: time.Now(),
		Metadata: make(map[string]string),
	}
} 