#!/bin/bash

# Script to test node connectivity to controller
# Usage: ./test-node-connection.sh [controller_url] [node_port]

set -e

CONTROLLER_URL=${1:-"http://localhost:9090"}
NODE_PORT=${2:-12345}

echo "Testing node connectivity to controller at $CONTROLLER_URL..."

# Check if controller is running
if ! curl -s -f "$CONTROLLER_URL/health" > /dev/null; then
  echo "Error: Controller is not running or not accessible at $CONTROLLER_URL"
  echo "Make sure the controller is running before starting nodes."
  exit 1
fi

echo "Controller is accessible."

# Test node registration
echo "Testing node registration..."
REGISTER_RESPONSE=$(curl -s -X POST "$CONTROLLER_URL/nodes/register" \
  -H "Content-Type: application/json" \
  -d "{\"address\":\"localhost:$NODE_PORT\"}")

if echo "$REGISTER_RESPONSE" | grep -q "id"; then
  echo "Node registration successful!"
  echo "Response: $REGISTER_RESPONSE"
else
  echo "Error: Node registration failed."
  echo "Response: $REGISTER_RESPONSE"
  exit 1
fi

echo "All tests passed! The node can connect to the controller."
echo "You can now start nodes using the manage-nodes.sh script."

exit 0
