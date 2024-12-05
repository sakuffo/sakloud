package cluster

import "net"

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

// SelectBestIP selects the most appropriate IP address for cluster communication
// Prefers private network IPs, falls back to public IPs, and avoids link-local and loopback
func SelectBestIP(addrs []net.Addr) net.IP {
    // First try to find a private network IP
    for _, addr := range addrs {
        if ipnet, ok := addr.(*net.IPNet); ok {
            if ip4 := ipnet.IP.To4(); ip4 != nil {
                if !isLinkLocal(ip4) && !isLoopback(ip4) && isPrivateNetwork(ip4) {
                    return ip4
                }
            }
        }
    }

    // If no private network IP found, try any non-link-local, non-loopback IP
    for _, addr := range addrs {
        if ipnet, ok := addr.(*net.IPNet); ok {
            if ip4 := ipnet.IP.To4(); ip4 != nil {
                if !isLinkLocal(ip4) && !isLoopback(ip4) {
                    return ip4
                }
            }
        }
    }

    return nil
} 