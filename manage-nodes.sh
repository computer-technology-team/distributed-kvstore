#!/usr/bin/env bash

# Script to manage distributed-kvstore nodes
# Usage: ./manage-nodes.sh [add|remove|list] [options]

set -e

# Default values
DEFAULT_CONTROLLER_URL="http://localhost:9090"
DEFAULT_NODE_PORT_START=12345
DEFAULT_NODE_COUNT=1
DEFAULT_NODE_NAME_PREFIX="node"
DEFAULT_IMAGE_NAME="kvstore-node"
DEFAULT_CONFIG_DIR="./config"

# Function to display usage information
show_usage() {
  echo "Usage: $0 [command] [options]"
  echo ""
  echo "Commands:"
  echo "  add     Add one or more nodes"
  echo "  remove  Remove one or more nodes"
  echo "  list    List running nodes"
  echo ""
  echo "Options for 'add':"
  echo "  --count N                Number of nodes to add (default: $DEFAULT_NODE_COUNT)"
  echo "  --port-start N           Starting port number (default: $DEFAULT_NODE_PORT_START)"
  echo "  --controller-url URL     Controller URL (default: $DEFAULT_CONTROLLER_URL)"
  echo ""
  echo "Options for 'remove':"
  echo "  --name NAME              Name of the node to remove"
  echo "  --all                    Remove all nodes"
  echo ""
  echo "Examples:"
  echo "  $0 add --count 3 --port-start 12345"
  echo "  $0 remove --name node12345"
  echo "  $0 remove --all"
  echo "  $0 list"
}

# Function to build the Docker image if it doesn't exist
ensure_image_exists() {
  if ! docker image inspect "$DEFAULT_IMAGE_NAME" &>/dev/null; then
    echo "Building Docker image $DEFAULT_IMAGE_NAME..."
    docker build -t "$DEFAULT_IMAGE_NAME" .
  fi
}

# Function to add nodes
add_nodes() {
  local count=$1
  local port_start=$2
  local controller_url=$3

  # Make sure the Docker image exists
  ensure_image_exists

  echo "Adding $count node(s) starting from port $port_start..."
  
  for ((i=0; i<count; i++)); do
    local port=$((port_start + i))
    local node_name="${DEFAULT_NODE_NAME_PREFIX}${port}"
    
    echo "Starting node $node_name on port $port..."
    
    # Start the node with Docker
    docker run -d \
      --name "$node_name" \
      --network host \
      -e DIST_KV_LOG_LEVEL=info \
      -e DIST_KV_NODE__HOST=0.0.0.0 \
      -e DIST_KV_NODE__PORT="$port" \
      -e DIST_KV_NODE__CONTROLLER_URL="$controller_url" \
      --health-cmd "curl -f http://localhost:$port/health || exit 1" \
      --health-interval 10s \
      --health-timeout 5s \
      --health-retries 3 \
      --health-start-period 5s \
      --restart unless-stopped \
      -v "$(pwd)/${DEFAULT_CONFIG_DIR}:/app/config" \
      "$DEFAULT_IMAGE_NAME" \
      ./kvstore servenode --config /app/config/node.yaml
    
    echo "Node $node_name started on port $port"
  done
}

# Function to remove nodes
remove_nodes() {
  if [ "$1" == "--all" ]; then
    echo "Removing all nodes..."
    
    # Get all node container names
    local nodes=$(docker ps --filter "name=${DEFAULT_NODE_NAME_PREFIX}" --format "{{.Names}}")
    
    if [ -z "$nodes" ]; then
      echo "No nodes found."
      return
    fi
    
    for node in $nodes; do
      echo "Stopping node $node..."
      docker stop "$node" && docker rm "$node"
    done
    
    echo "All nodes removed."
  else
    local node_name=$1
    
    echo "Removing node $node_name..."
    
    # Check if the node exists
    if ! docker ps --filter "name=$node_name" --format "{{.Names}}" | grep -q "$node_name"; then
      echo "Node $node_name not found."
      return 1
    fi
    
    # Stop and remove the node
    docker stop "$node_name" && docker rm "$node_name"
    
    echo "Node $node_name removed."
  fi
}

# Function to list nodes
list_nodes() {
  echo "Listing all nodes..."
  
  # Get all node container information
  docker ps --filter "name=${DEFAULT_NODE_NAME_PREFIX}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
}

# Main script logic
if [ $# -lt 1 ]; then
  show_usage
  exit 1
fi

command=$1
shift

case $command in
  "add")
    # Default values
    count=$DEFAULT_NODE_COUNT
    port_start=$DEFAULT_NODE_PORT_START
    controller_url=$DEFAULT_CONTROLLER_URL
    
    # Parse options
    while [ $# -gt 0 ]; do
      case $1 in
        --count)
          count=$2
          shift 2
          ;;
        --port-start)
          port_start=$2
          shift 2
          ;;
        --controller-url)
          controller_url=$2
          shift 2
          ;;
        *)
          echo "Unknown option: $1"
          show_usage
          exit 1
          ;;
      esac
    done
    
    add_nodes $count $port_start $controller_url
    ;;
    
  "remove")
    if [ $# -lt 1 ]; then
      echo "Missing options for 'remove' command."
      show_usage
      exit 1
    fi
    
    if [ "$1" == "--all" ]; then
      remove_nodes "--all"
    elif [ "$1" == "--name" ]; then
      if [ -z "$2" ]; then
        echo "Missing node name."
        show_usage
        exit 1
      fi
      remove_nodes $2
    else
      echo "Unknown option: $1"
      show_usage
      exit 1
    fi
    ;;
    
  "list")
    list_nodes
    ;;
    
  *)
    echo "Unknown command: $command"
    show_usage
    exit 1
    ;;
esac

exit 0
