#!/bin/bash
# Remove quarantine attribute from macOS
xattr -d com.apple.quarantine kubectl-flex-to-csi 2>/dev/null || true
xattr -d com.apple.quarantine run.command 2>/dev/null || true
chmod +x kubectl-flex-to-csi
chmod +x run.command
echo 'Installation complete! You can now run ./kubectl-flex-to-csi or ./run.command'
