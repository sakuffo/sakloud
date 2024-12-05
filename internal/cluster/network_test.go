package cluster

import (
    "net"
    "testing"
)

func TestIPValidation(t *testing.T) {
    tests := []struct {
        name           string
        ip            string
        isLinkLocal   bool
        isLoopback    bool
        isPrivateNet  bool
    }{
        {
            name:          "link-local address",
            ip:           "169.254.1.1",
            isLinkLocal:  true,
            isLoopback:   false,
            isPrivateNet: false,
        },
        {
            name:          "loopback address",
            ip:           "127.0.0.1",
            isLinkLocal:  false,
            isLoopback:   true,
            isPrivateNet: false,
        },
        {
            name:          "private network (192.168.x.x)",
            ip:           "192.168.1.1",
            isLinkLocal:  false,
            isLoopback:   false,
            isPrivateNet: true,
        },
        {
            name:          "private network (10.x.x.x)",
            ip:           "10.0.0.1",
            isLinkLocal:  false,
            isLoopback:   false,
            isPrivateNet: true,
        },
        {
            name:          "private network (172.16-31.x.x)",
            ip:           "172.16.0.1",
            isLinkLocal:  false,
            isLoopback:   false,
            isPrivateNet: true,
        },
        {
            name:          "public address",
            ip:           "8.8.8.8",
            isLinkLocal:  false,
            isLoopback:   false,
            isPrivateNet: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ip := net.ParseIP(tt.ip)
            if ip == nil {
                t.Fatalf("Failed to parse IP: %s", tt.ip)
            }

            if got := isLinkLocal(ip); got != tt.isLinkLocal {
                t.Errorf("isLinkLocal() = %v, want %v", got, tt.isLinkLocal)
            }

            if got := isLoopback(ip); got != tt.isLoopback {
                t.Errorf("isLoopback() = %v, want %v", got, tt.isLoopback)
            }

            if got := isPrivateNetwork(ip); got != tt.isPrivateNet {
                t.Errorf("isPrivateNetwork() = %v, want %v", got, tt.isPrivateNet)
            }
        })
    }
}

func TestSelectBestIP(t *testing.T) {
    tests := []struct {
        name     string
        addrs    []net.Addr
        wantIP   string
        wantNil  bool
    }{
        {
            name: "prefer private over public",
            addrs: []net.Addr{
                &net.IPNet{IP: net.ParseIP("8.8.8.8")},
                &net.IPNet{IP: net.ParseIP("192.168.1.1")},
            },
            wantIP: "192.168.1.1",
        },
        {
            name: "skip link-local and loopback",
            addrs: []net.Addr{
                &net.IPNet{IP: net.ParseIP("169.254.1.1")},
                &net.IPNet{IP: net.ParseIP("127.0.0.1")},
                &net.IPNet{IP: net.ParseIP("8.8.8.8")},
            },
            wantIP: "8.8.8.8",
        },
        {
            name: "no suitable addresses",
            addrs: []net.Addr{
                &net.IPNet{IP: net.ParseIP("169.254.1.1")},
                &net.IPNet{IP: net.ParseIP("127.0.0.1")},
            },
            wantNil: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := SelectBestIP(tt.addrs)
            if tt.wantNil {
                if got != nil {
                    t.Errorf("SelectBestIP() = %v, want nil", got)
                }
                return
            }
            if got == nil {
                t.Fatal("SelectBestIP() = nil, want IP")
            }
            if got.String() != tt.wantIP {
                t.Errorf("SelectBestIP() = %v, want %v", got, tt.wantIP)
            }
        })
    }
} 