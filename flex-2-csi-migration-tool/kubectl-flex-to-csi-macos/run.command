#!/bin/bash

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Change to the script directory
cd "$SCRIPT_DIR"

# Check if kubeconfig exists
if [ ! -f "kubeconfig/config" ]; then
    echo "Error: kubeconfig/config file not found!"
    echo "Please place your kubeconfig file at: $SCRIPT_DIR/kubeconfig/config"
    echo ""
    read -p "Press Enter to exit..."
    exit 1
fi

# Set KUBECONFIG environment variable
export KUBECONFIG="$SCRIPT_DIR/kubeconfig/config"

# Run the kubectl-flex-to-csi tool
./kubectl-flex-to-csi

# Keep terminal open
read -p "Press Enter to exit..."