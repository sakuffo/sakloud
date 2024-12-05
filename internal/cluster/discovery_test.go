package cluster

import (
    "context"
    "net"
    "testing"
    "time"

    "github.com/grandcat/zeroconf"
    "github.com/sakuffo/sakloud/internal/config"
    "github.com/sakuffo/sakloud/internal/logger"
)

func TestHandleDiscoveredNode(t *testing.T) {
    tests := []struct {
        name string
        setup func(t *testing.T) (*Cluster, *zeroconf.ServiceEntry)
        check func(t *testing.T, c *Cluster, entry *zeroconf.ServiceEntry)
    }{
        {
            name: "valid node is added to cluster",
            setup: func(t *testing.T) (*Cluster, *zeroconf.ServiceEntry) {
                c := newTestCluster(t)
                entry := zeroconf.NewServiceEntry("test-node", "_sakloud._tcp", "local.")
                entry.Port = 7946
                entry.AddrIPv4 = []net.IP{net.ParseIP("192.168.1.1")}
                entry.Text = []string{"id=test-id-1"}
                return c, entry
            },
            check: func(t *testing.T, c *Cluster, entry *zeroconf.ServiceEntry) {
                node, exists := c.GetNode("test-id-1")
                if !exists {
                    t.Fatal("node should exist in cluster")
                }
                if !node.Address.Equal(net.ParseIP("192.168.1.1")) {
                    t.Errorf("got address %v, want %v", node.Address, net.ParseIP("192.168.1.1"))
                }
            },
        },
        {
            name: "link-local address is skipped",
            setup: func(t *testing.T) (*Cluster, *zeroconf.ServiceEntry) {
                c := newTestCluster(t)
                entry := zeroconf.NewServiceEntry("test-node", "_sakloud._tcp", "local.")
                entry.Port = 7946
                entry.AddrIPv4 = []net.IP{
                    net.ParseIP("169.254.1.1"),
                    net.ParseIP("169.254.2.2"),
                }
                entry.Text = []string{"id=test-id-2"}
                return c, entry
            },
            check: func(t *testing.T, c *Cluster, entry *zeroconf.ServiceEntry) {
                _, exists := c.GetNode("test-id-2")
                if exists {
                    t.Error("node with only link-local addresses should not be added")
                }
            },
        },
        {
            name: "node without ID is rejected",
            setup: func(t *testing.T) (*Cluster, *zeroconf.ServiceEntry) {
                c := newTestCluster(t)
                entry := zeroconf.NewServiceEntry("test-node", "_sakloud._tcp", "local.")
                entry.Port = 7946
                entry.AddrIPv4 = []net.IP{net.ParseIP("192.168.1.1")}
                entry.Text = []string{}
                return c, entry
            },
            check: func(t *testing.T, c *Cluster, entry *zeroconf.ServiceEntry) {
                if len(c.nodes) != 0 {
                    t.Error("cluster should be empty when node has no ID")
                }
            },
        },
        {
            name: "metadata is properly captured",
            setup: func(t *testing.T) (*Cluster, *zeroconf.ServiceEntry) {
                c := newTestCluster(t)
                entry := zeroconf.NewServiceEntry("test-node", "_sakloud._tcp", "local.")
                entry.Port = 7946
                entry.AddrIPv4 = []net.IP{net.ParseIP("192.168.1.1")}
                entry.Text = []string{
                    "id=test-id-4",
                    "version=1.0",
                    "region=us-west",
                }
                return c, entry
            },
            check: func(t *testing.T, c *Cluster, entry *zeroconf.ServiceEntry) {
                node, exists := c.GetNode("test-id-4")
                if !exists {
                    t.Fatal("node should exist in cluster")
                }
                expectedMetadata := map[string]string{
                    "version": "1.0",
                    "region":  "us-west",
                }
                for k, v := range expectedMetadata {
                    if node.Metadata[k] != v {
                        t.Errorf("metadata[%s] = %v, want %v", k, node.Metadata[k], v)
                    }
                }
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            c, entry := tt.setup(t)
            c.handleDiscoveredNode(entry)
            tt.check(t, c, entry)
        })
    }
}

// newTestCluster creates a new cluster for testing
func newTestCluster(t *testing.T) *Cluster {
    t.Helper()
    localNode := NewNode("local-test", net.ParseIP("127.0.0.1"), 7946)
    return NewCluster(
        context.Background(),
        localNode,
        newTestLogger(t),
        &config.Config{
            ServiceName:           "test-service",
            DiscoveryPort:        7946,
            HealthInterval:       time.Second,
            SuspectTimeout:       time.Second * 3,
            DeadTimeout:          time.Second * 6,
            NodeRemovalDelay:     time.Second * 9,
            DiscoveryRetryInterval: time.Second,
            DiscoveryTimeout:     time.Second,
            DiscoveryBufferSize:  4,
        },
    )
}

// newTestLogger creates a new logger for testing
func newTestLogger(t *testing.T) logger.Logger {
    t.Helper()
    return &testLogger{t: t}
}

type testLogger struct {
    t *testing.T
}

func (l *testLogger) Debug(format string, v ...interface{}) { l.t.Logf("[DEBUG] "+format, v...) }
func (l *testLogger) Info(format string, v ...interface{})  { l.t.Logf("[INFO] "+format, v...) }
func (l *testLogger) Warn(format string, v ...interface{})  { l.t.Logf("[WARN] "+format, v...) }
func (l *testLogger) Error(format string, v ...interface{}) { l.t.Logf("[ERROR] "+format, v...) }
func (l *testLogger) Fatal(format string, v ...interface{}) { l.t.Fatalf("[FATAL] "+format, v...) } 