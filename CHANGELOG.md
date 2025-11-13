# Changelog

All notable changes to SSH Manager (sshm) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Updated Go version requirement to 1.24+
- Updated dependencies:
  - Bubble Tea to v1.3.10
  - Bubbles to v0.21.0
  - Lip Gloss to v1.1.0
  - golang.org/x/crypto to v0.44.0
  - golang.org/x/term to v0.37.0
  - pkg/sftp to v1.13.10
  - go-scp to v1.5.0
  - containerd/console to v1.0.5

### Added
- Comprehensive documentation suite
  - Installation guide
  - User guide
  - Architecture documentation
  - Security documentation
  - Contributing guidelines

## [1.0.0] - 2024

### Added
- **Core Features**
  - Terminal-based UI using Bubble Tea framework
  - Secure credential management with AES-256-GCM encryption
  - SSH connection management
  - Password and SSH key authentication support
  - Multiple color themes
  - Cross-platform support (Linux, macOS, Windows - x64 & ARM64)

- **File Transfer**
  - Dual-pane file transfer interface
  - SFTP and SCP support
  - Batch file operations
  - Directory navigation
  - File/directory creation, deletion, and renaming

- **Credential Management**
  - Encrypted password storage
  - SSH key storage (imported or referenced)
  - Host configuration management
  - CRUD operations for all credential types

- **Cloud Synchronization**
  - Optional cloud sync with sshm.io
  - API key encryption
  - Automatic backups before sync
  - Local-only mode support

- **Mouse Support**
  - Click selection for all UI elements
  - Double-click actions (connect, navigate)
  - Scroll wheel support
  - Accurate click position detection

- **Terminal Features**
  - Interactive SSH sessions
  - Terminal resize handling
  - Keep-alive for long sessions
  - xterm-256color support

- **Security**
  - AES-256-GCM authenticated encryption
  - Master password-based key derivation
  - Known hosts verification
  - Secure file permissions (0600)
  - Encrypted API key storage

### Changed
- N/A (Initial release)

### Deprecated
- N/A (Initial release)

### Removed
- N/A (Initial release)

### Fixed
- N/A (Initial release)

### Security
- N/A (Initial release)

---

## Release Notes

### Version 1.0.0

This is the initial release of SSH Manager (sshm), providing a complete, secure SSH connection management solution with the following highlights:

**For Users:**
- Easy-to-use terminal interface with keyboard and mouse support
- Secure storage of SSH credentials with military-grade encryption
- Quick access to frequently used servers
- Built-in file transfer capabilities

**For Organizations:**
- Optional cloud synchronization for team credential sharing
- Self-hostable backend (sshm.io is open source)
- Comprehensive audit trail capabilities
- Compliance-friendly security model

**For Developers:**
- Clean, modular architecture
- Well-documented codebase
- Extensible design patterns
- Active development and community support

---

## How to Read This Changelog

- **Added**: New features
- **Changed**: Changes in existing functionality
- **Deprecated**: Soon-to-be-removed features
- **Removed**: Removed features
- **Fixed**: Bug fixes
- **Security**: Security vulnerability fixes

[Unreleased]: https://github.com/kofany/sshmanager/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/kofany/sshmanager/releases/tag/v1.0.0
