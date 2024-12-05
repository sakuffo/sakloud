# Sakloud

A distributed cloud storage system written in Go.

## Features

- Automatic node discovery using mDNS
- Health checking and node state management
- Event-based architecture for cluster state changes
- Graceful shutdown handling

## Getting Started

### Prerequisites

- Go 1.19 or later
- Working network connection for mDNS discovery

### Installation

```bash
go get github.com/sakuffo/sakloud
```

### Running

```bash
go run cmd/sakloud/main.go
```

## Project Structure

```
.
├── cmd
│   └── sakloud
│       └── main.go      # Contains only the main function and basic setup
├── internal
│   ├── cluster
│   │   ├── cluster.go   # Cluster type and core cluster management
│   │   ├── node.go      # Node type and node-related operations
│   │   ├── discovery.go # mDNS discovery related code
│   │   ├── events.go    # Event types and event handling
│   │   └── health.go    # Health checking logic
│   └── config
│       └── config.go    # Configuration types and loading
├── go.mod
└── README.md
```

## License

MIT License 