# kubectl-flex-to-csi Migration Tool

## Quick Start

### Prerequisites
- A valid Kubernetes kubeconfig file with access to your cluster

### Setup Instructions

1. **Place your kubeconfig file**
   - Copy your kubeconfig file to the `kubeconfig/` directory
   - Rename it to `config` (no file extension)
   - Path should be: `kubeconfig/config`

2. **Run the tool**
   - **macOS/Linux**: Double-click `run.command` (or `run.sh` on Linux)
   - **Windows**: Double-click `run.bat`

### Manual Execution

If you prefer to run the tool manually:

```bash
# Set your kubeconfig
export KUBECONFIG=/path/to/your/kubeconfig

# Run the tool
./kubectl-flex-to-csi
```

### Troubleshooting

**"kubeconfig not found" error:**
- Ensure your kubeconfig file is placed at `kubeconfig/config`
- Check that the file has the correct permissions (readable)

**Permission denied (macOS/Linux):**
```bash
chmod +x run.command  # or run.sh on Linux
chmod +x kubectl-flex-to-csi
```

### Support

For more information, see the main README.md file or visit the project repository.