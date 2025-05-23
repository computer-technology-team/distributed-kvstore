# Distributed Key-Value Store

A scalable and resilient distributed key-value store built with modern Go practices.

## Overview

This project implements a distributed key-value store designed for high availability, fault tolerance, and horizontal scalability. It provides a simple yet powerful interface for storing and retrieving data across a cluster of nodes.

## Technology Stack

### Core Libraries

- **[Chi Router](https://github.com/go-chi/chi)**: Lightweight, idiomatic HTTP router for building Go HTTP services
- **[Cobra](https://github.com/spf13/cobra)**: Powerful CLI application framework
- **[Viper](https://github.com/spf13/viper)**: Complete configuration solution for Go applications
- **[slog](https://pkg.go.dev/log/slog)**: Structured logging for Go applications
- **[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)**: OpenAPI code generator for both internal and public APIs

## Project Structure

```
distributed-kvstore/
├── cmd/                  # Command-line interface definitions
├── config/               # Configuration management
├── internal/             # Internal packages
│   └── kvstore/          # Core key-value store implementation
├── api/                  # API definitions (OpenAPI specs)
├── .github/workflows/    # CI/CD pipelines
└── main.go               # Application entry point
```

## Getting Started

### Prerequisites

- Go 1.24 or higher

### Installation

```bash
# Clone the repository
git clone https://github.com/computer-technology-team/distributed-kvstore.git
cd distributed-kvstore

# Build the project
make build
```

### Running the Service

```bash
# Start the controller and load balancer
docker compose up -d

# Start a single node
./kvstore servenode

# For more options
./kvstore --help
```

### Managing Nodes with Docker

The project includes a script to help manage nodes in a Docker environment. This allows you to add or remove nodes on demand without having to manually configure each one.

```bash
# View usage information
./manage-nodes.sh

# Add 3 nodes starting from port 12345
./manage-nodes.sh add --count 3 --port-start 12345

# List all running nodes
./manage-nodes.sh list

# Remove a specific node
./manage-nodes.sh remove --name node12345

# Remove all nodes
./manage-nodes.sh remove --all
```

Each node will automatically:

1. Build the Docker image if it doesn't exist
2. Run with a unique port and container name
3. Register with the controller upon startup
4. Include health checks to ensure it's functioning properly
5. Use the host network for optimal performance

The script makes it easy to scale your cluster up or down based on your needs.

## Configuration

Configuration is managed through Viper, which supports multiple formats (YAML, JSON, TOML) and sources (files, environment variables, command-line flags).

Example configuration:

```yaml
# Server configuration
server:
  port: 8080
  host: 0.0.0.0

# Client configuration
client:
  server_url: http://localhost:8080
```

You can specify a configuration file using the `--config` flag:

```bash
./kvstore --config /path/to/config.yaml client get mykey
```

Or use environment variables:

```bash
DIST_KV__CLIENT__SERVER_URL=http://localhost:8080 ./kvstore client get mykey
```

## Development

### API Generation

We use oapi-codegen to generate server and client code from OpenAPI specifications:

```bash
# Generate  code
make generate
```

### Building

```bash
# Build the binary
make build

```
