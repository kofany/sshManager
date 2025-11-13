# Installation Guide

This guide provides detailed installation instructions for SSH Manager (sshm) across different platforms and scenarios.

## Table of Contents

- [System Requirements](#system-requirements)
- [Installation Methods](#installation-methods)
  - [From Source](#from-source)
  - [Pre-built Binaries](#pre-built-binaries)
  - [Using Go Install](#using-go-install)
- [Platform-Specific Instructions](#platform-specific-instructions)
  - [Linux](#linux)
  - [macOS](#macos)
  - [Windows](#windows)
- [Post-Installation](#post-installation)
- [Updating](#updating)
- [Uninstallation](#uninstallation)

## System Requirements

### Minimum Requirements
- **Operating System**: Linux, macOS, or Windows (x64 or ARM64)
- **Go Version**: 1.23 or higher (for building from source)
- **Terminal**: UTF-8 compatible terminal with color support
- **Disk Space**: ~10 MB for binary, additional space for configuration
- **Memory**: ~50 MB RAM during operation

### Recommended
- Terminal emulator with xterm-256color support
- Mouse support in terminal (for enhanced interaction)
- SSH client installed on system (for debugging)

### Compatible Terminals
- **Linux**: GNOME Terminal, Konsole, Alacritty, Kitty, Terminator
- **macOS**: Terminal.app, iTerm2, Alacritty, Kitty
- **Windows**: Windows Terminal, ConEmu, Alacritty

## Installation Methods

### From Source

This is the recommended method for developers or users who want the latest features.

#### Step 1: Install Go

If you don't have Go installed:

**Linux (Ubuntu/Debian)**
```bash
sudo apt update
sudo apt install golang-go
```

**Linux (Fedora)**
```bash
sudo dnf install golang
```

**macOS**
```bash
brew install go
```

**Windows**
Download from [go.dev](https://go.dev/dl/) and run the installer.

#### Step 2: Clone Repository

```bash
git clone https://github.com/kofany/sshmanager.git
cd sshmanager
```

#### Step 3: Build

```bash
# Simple build
go build -o sshm ./cmd/sshm/main.go

# Or use the build script for optimized binary
chmod +x build.sh
./build.sh
```

#### Step 4: Install (Optional)

```bash
# Linux/macOS - system-wide installation
sudo mv sshm /usr/local/bin/

# Linux/macOS - user installation
mkdir -p ~/.local/bin
mv sshm ~/.local/bin/
# Add ~/.local/bin to PATH if not already

# Windows
# Move sshm.exe to a directory in your PATH
```

### Pre-built Binaries

Pre-built binaries are available for all supported platforms.

#### Step 1: Download

Download the appropriate binary for your platform from the releases page:

```bash
# Example for Linux amd64
wget https://github.com/kofany/sshmanager/releases/latest/download/sshm_linux_amd64
chmod +x sshm_linux_amd64
mv sshm_linux_amd64 sshm
```

#### Step 2: Install

```bash
# Linux/macOS
sudo mv sshm /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/kofany/sshmanager/cmd/sshm@latest
```

This installs the binary to `$GOPATH/bin` (typically `~/go/bin`).

## Platform-Specific Instructions

### Linux

#### Debian/Ubuntu

```bash
# Install dependencies
sudo apt update
sudo apt install git golang-go

# Clone and build
git clone https://github.com/kofany/sshmanager.git
cd sshmanager
go build -o sshm ./cmd/sshm/main.go

# Install system-wide
sudo install -m 755 sshm /usr/local/bin/sshm

# Verify installation
sshm --version
```

#### Fedora/RHEL/CentOS

```bash
# Install dependencies
sudo dnf install git golang

# Clone and build
git clone https://github.com/kofany/sshmanager.git
cd sshmanager
go build -o sshm ./cmd/sshm/main.go

# Install system-wide
sudo install -m 755 sshm /usr/local/bin/sshm
```

#### Arch Linux

```bash
# Install dependencies
sudo pacman -S git go

# Clone and build
git clone https://github.com/kofany/sshmanager.git
cd sshmanager
go build -o sshm ./cmd/sshm/main.go

# Install system-wide
sudo install -Dm755 sshm /usr/local/bin/sshm
```

### macOS

#### Using Homebrew (if Go already installed)

```bash
# Install Go if needed
brew install go

# Clone and build
git clone https://github.com/kofany/sshmanager.git
cd sshmanager
go build -o sshm ./cmd/sshm/main.go

# Install
sudo mv sshm /usr/local/bin/
```

#### Universal Binary for Apple Silicon

```bash
# Clone repository
git clone https://github.com/kofany/sshmanager.git
cd sshmanager

# Build for Apple Silicon
GOARCH=arm64 GOOS=darwin go build -o sshm ./cmd/sshm/main.go

# Or use the build script
./build.sh
# Binary will be in build/sshm_darwin_arm64
```

### Windows

#### Using PowerShell

```powershell
# Install Go from https://go.dev/dl/ first

# Clone repository
git clone https://github.com/kofany/sshmanager.git
cd sshmanager

# Build
go build -o sshm.exe .\cmd\sshm\main.go

# Move to a directory in PATH (example: C:\Program Files\sshm)
New-Item -ItemType Directory -Path "C:\Program Files\sshm" -Force
Move-Item sshm.exe "C:\Program Files\sshm\"

# Add to PATH
[Environment]::SetEnvironmentVariable(
    "Path",
    [Environment]::GetEnvironmentVariable("Path", "User") + ";C:\Program Files\sshm",
    "User"
)
```

#### Using Windows Terminal

For best experience, use Windows Terminal:
1. Install from Microsoft Store
2. Configure UTF-8 support
3. Enable mouse support in settings

## Post-Installation

### Verify Installation

```bash
# Check if sshm is in PATH
which sshm  # Linux/macOS
where sshm  # Windows

# Try running
sshm
```

### First Run Setup

1. Run `sshm` for the first time
2. Create a master password (choose strong password)
3. Either enter API key or press ESC for local mode
4. Start using the application

### Configuration Directory

The application will automatically create the configuration directory:

- **Linux/macOS**: `~/.config/sshm/`
- **Windows**: `%USERPROFILE%\.config\sshm\`

### Set Permissions (Linux/macOS)

```bash
# Ensure proper permissions for config directory
chmod 700 ~/.config/sshm
chmod 600 ~/.config/sshm/*
```

## Updating

### From Source

```bash
cd sshmanager
git pull
go build -o sshm ./cmd/sshm/main.go
sudo mv sshm /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/kofany/sshmanager/cmd/sshm@latest
```

### Pre-built Binary

Download the latest release and replace the existing binary.

## Uninstallation

### Remove Binary

```bash
# Linux/macOS
sudo rm /usr/local/bin/sshm

# Windows
Remove-Item "C:\Program Files\sshm\sshm.exe"
```

### Remove Configuration (Optional)

**Warning**: This will delete all your saved hosts, passwords, and keys.

```bash
# Linux/macOS
rm -rf ~/.config/sshm

# Windows (PowerShell)
Remove-Item -Recurse -Force "$env:USERPROFILE\.config\sshm"
```

### Backup Before Uninstall

```bash
# Linux/macOS - create backup
tar -czf sshm-backup-$(date +%Y%m%d).tar.gz ~/.config/sshm

# Windows (PowerShell)
Compress-Archive -Path "$env:USERPROFILE\.config\sshm" -DestinationPath "sshm-backup-$(Get-Date -Format 'yyyyMMdd').zip"
```

## Troubleshooting Installation

### Problem: "command not found" after installation

**Solution**: Ensure the installation directory is in your PATH.

```bash
# Linux/macOS - Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:$HOME/.local/bin"

# Reload shell configuration
source ~/.bashrc  # or ~/.zshrc
```

### Problem: Permission denied when running

**Solution**: Make the binary executable.

```bash
chmod +x sshm
```

### Problem: Build fails with "go: command not found"

**Solution**: Install Go from [go.dev](https://go.dev/dl/) and ensure it's in your PATH.

### Problem: Terminal doesn't support colors

**Solution**: Set terminal type environment variable.

```bash
export TERM=xterm-256color
```

### Problem: Windows Defender blocks the executable

**Solution**: Add an exclusion for the sshm binary in Windows Security settings.

## Getting Help

If you encounter issues during installation:

1. Check the [Troubleshooting](../README.md#troubleshooting) section
2. Open an issue on [GitHub](https://github.com/kofany/sshmanager/issues)
3. Contact support at [j@dabrowski.biz](mailto:j@dabrowski.biz)

---

[Back to Main Documentation](../README.md)
