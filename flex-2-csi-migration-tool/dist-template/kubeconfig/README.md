# Kubeconfig Directory

## Instructions

Place your Kubernetes kubeconfig file in this directory and rename it to `config` (no file extension).

### Example:

```bash
cp ~/.kube/config ./config
```

### Requirements:

- The file must be named exactly `config`
- The kubeconfig must have valid credentials for your cluster
- Ensure the file has appropriate read permissions

### Security Note:

⚠️ **Important**: Never commit your kubeconfig file to version control. This directory is included in `.gitignore` to prevent accidental commits.

The kubeconfig file contains sensitive credentials that provide access to your Kubernetes cluster.