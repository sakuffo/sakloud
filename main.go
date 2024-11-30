package main

import (
	"net"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
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

// Cluster manages the state of all nodes in the cluster
type Cluster struct {
	mutex       sync.RWMutex
	localNode   *Node
	nodes       map[string]*Node
	subscribers []ClusterEventFunc
	server      *mdns.Server
}

// ClusterEvent represents a change in the cluster state
type ClusterEvent struct {
	Type      EventType
	Node      *Node
	Timestamp time.Time
}

// EventType defines the type of cluster event
type EventType int

const (
	NodeJoin EventType = iota
	NodeLeave
	NodeStateChange
)

// ClusterEventFunc is a callback function type for cluster events
type ClusterEventFunc func(event ClusterEvent)

// NewCluster initializes a new cluster with the local node
func NewCluster(localNode *Node) *Cluster {
	return &Cluster{
		localNode: localNode,
		nodes:     make(map[string]*Node),
	}
}

// AddNode adds a node to the cluster

func (c *Cluster) AddNode(node *Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodes[node.ID] = node
	c.notifySubscribers(ClusterEvent{
		Type:      NodeJoin,
		Node:      node,
		Timestamp: time.Now(),
	})
}

// Subscribe adds an event listener

func (c *Cluster) Subscribe(fn ClusterEventFunc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.subscribers = append(c.subscribers, fn)
}

// notifySubscribers sends events to all subscribers
func (c *Cluster) notifySubscribers(event ClusterEvent) {
	for _, fn := range c.subscribers {
		go fn(event)
	}
}

// StartDiscovery initializes mDNS service and discovery
func (c *Cluster) StartDiscovery(serviceName string) error {
	// Create mDNS config
	config := &mdns.Config{
		Service: serviceName,
		Domain:  "local.",
		Port:    c.localNode.Port,
		IP:      []net.IP{c.localNode.Address},
	}

	// Create mDNS server
	server, err := mdns.NewServer(config)
	if err != nil {
		return err
	}
	c.server = server

	// Start discovery
	go c.discover(serviceName)
	return nil
}

// discover continuously looks for other nodes
func (c *Cluster) discover(serviceName string) {
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	go func() {
		for entry := range entriesCh {
			// Skip our own entry
			if entry.AddrV4.String() == c.localNode.Address.String() {
				continue
			}

			node := &Node{
				ID:       entry.Name,
				Name:     entry.Name,
				Address:  entry.AddrV4,
				Port:     entry.Port,
				State:    Alive,
				LastSeen: time.Now(),
				Metadata: make(map[string]string),
			}

			c.AddNode(node)
		}
	}()

	// Query for services
	params := mdns.DefaultParams(serviceName)
	params.Entries = entriesCh
	params.WantUnicastResponse = true

	// Continuously look for new nodes
	for {
		mdns.Query(params)
		time.Sleep(time.Second * 10) // Query every 10 seconds
	}
}
