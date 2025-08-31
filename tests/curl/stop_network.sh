#!/bin/bash

echo "Stopping blockchain network..."

# Kill all node processes
pkill -f "go run.*main.go"

# Wait a moment for cleanup
sleep 2

# Check if any processes are still running
REMAINING=$(pgrep -f "go run.*main.go" | wc -l)
if [ $REMAINING -eq 0 ]; then
    echo "All nodes stopped successfully."
else
    echo "Warning: $REMAINING node processes may still be running."
    echo "You may need to run 'pkill -9 -f \"go run.*main.go\"' to force kill them."
fi