# macOS Installation Guide

## ⚠️ Important: macOS Security Notice

macOS may block the `kubectl-flex-to-csi` binary because it's not code-signed by Apple. This is normal for open-source tools. Follow the instructions below to safely run the tool.

## Quick Installation (Recommended)

After extracting the zip file, run the included installation script:

```bash
cd kubectl-flex-to-csi-macos
./INSTALL.sh
```

This script will:
- Remove the quarantine attribute from the binary
- Set proper executable permissions
- Prepare the tool for use

## Manual Installation

If you prefer to do it manually, follow these steps:

### Method 1: Remove Quarantine Attribute (Recommended)

```bash
cd kubectl-flex-to-csi-macos
xattr -d com.apple.quarantine kubectl-flex-to-csi
xattr -d com.apple.quarantine run.command
chmod +x kubectl-flex-to-csi
chmod +x run.command
```

### Method 2: Using System Preferences

1. Try to run the binary:
   ```bash
   ./kubectl-flex-to-csi --help
   ```

2. macOS will show a security warning. Click **"Cancel"**

3. Open **System Preferences** → **Security & Privacy** → **General** tab

4. You'll see a message: *"kubectl-flex-to-csi was blocked from use because it is not from an identified developer"*

5. Click **"Allow Anyway"**

6. Try running the binary again:
   ```bash
   ./kubectl-flex-to-csi --help
   ```

7. Click **"Open"** when prompted

### Method 3: Right-Click Method

1. In Finder, navigate to the `kubectl-flex-to-csi-macos` folder

2. **Right-click** (or Control-click) on `kubectl-flex-to-csi`

3. Select **"Open"** from the menu

4. Click **"Open"** in the security dialog

5. The binary is now trusted and can be run from the terminal

## Verification

After installation, verify the tool works:

```bash
./kubectl-flex-to-csi --help
```

You should see the help message without any security warnings.

## Why This Happens

macOS Gatekeeper blocks unsigned binaries to protect users from malware. This tool is:
- ✅ Open source (you can review the code)
- ✅ Built from trusted sources
- ✅ Safe to use

However, it's not code-signed because:
- Code signing requires an Apple Developer account ($99/year)
- This is a free, open-source tool

## Alternative: Build from Source

If you prefer, you can build the binary yourself:

```bash
git clone https://github.com/balraj111/ibmcloud-object-storage-plugin.git
cd ibmcloud-object-storage-plugin/flex-2-csi-migration-tool
make build
./kubectl-flex-to-csi --help
```

## Support

If you encounter issues, please:
1. Check that you've followed the installation steps correctly
2. Verify your macOS version is supported (macOS 10.15+)
3. Open an issue on GitHub with details about the error

## Security Note

The quarantine attribute is a macOS security feature that marks files downloaded from the internet. Removing it is safe when you trust the source (this official repository).