# Architecture Documentation

This document provides an in-depth overview of the SSH Manager's architecture, design patterns, and internal workings.

## Table of Contents

- [High-Level Architecture](#high-level-architecture)
- [Technology Stack](#technology-stack)
- [Project Structure](#project-structure)
- [Core Components](#core-components)
  - [Entry Point](#entry-point)
  - [Configuration Management](#configuration-management)
  - [Encryption System](#encryption-system)
  - [SSH Client](#ssh-client)
  - [UI Framework](#ui-framework)
  - [Cloud Synchronization](#cloud-synchronization)
- [Data Flow](#data-flow)
- [State Management](#state-management)
- [Security Architecture](#security-architecture)
- [File System Layout](#file-system-layout)
- [Extension Points](#extension-points)
- [Performance Considerations](#performance-considerations)
- [Design Patterns](#design-patterns)

## High-Level Architecture

SSH Manager follows a modular, layered architecture:

```
┌─────────────────────────────────────────────────────────┐
│                    Presentation Layer                    │
│  (Bubble Tea Views: Main, Edit, Transfer, Prompts)      │
└───────────────────┬─────────────────────────────────────┘
                    │
┌───────────────────┴─────────────────────────────────────┐
│                   Application Layer                      │
│    (UI Model, State Management, Message Handling)       │
└───────────────────┬─────────────────────────────────────┘
                    │
┌───────────────────┴─────────────────────────────────────┐
│                    Business Layer                        │
│  (Config Manager, SSH Client, SFTP, Crypto, Sync)       │
└───────────────────┬─────────────────────────────────────┘
                    │
┌───────────────────┴─────────────────────────────────────┐
│                      Data Layer                          │
│    (JSON Config Files, Encrypted Keys, API Client)      │
└─────────────────────────────────────────────────────────┘
```

## Technology Stack

### Core Languages and Frameworks
- **Go 1.23**: Modern, efficient, compiled language with excellent concurrency support
- **Bubble Tea**: Elm-inspired TUI framework for building interactive terminal applications
- **Lip Gloss**: CSS-like styling framework for terminal UIs

### Key Libraries
- **golang.org/x/crypto**: SSH client implementation, AES encryption
- **pkg/sftp**: SFTP client for secure file transfers
- **go-scp**: SCP implementation for file transfers
- **golang.org/x/term**: Terminal handling and control

### External Services
- **sshm.io API**: Optional cloud synchronization (REST API)

## Project Structure

```
sshmanager/
├── cmd/
│   └── sshm/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── crypto/
│   │   └── crypto.go            # Encryption/decryption utilities
│   ├── models/
│   │   ├── config.go            # Data models
│   │   ├── host.go              # Host model
│   │   ├── password.go          # Password model
│   │   └── key.go               # SSH key model
│   ├── ssh/
│   │   ├── client.go            # SSH client implementation
│   │   ├── session.go           # SSH session handling
│   │   ├── sftp.go              # SFTP operations
│   │   └── known_hosts.go       # Host key verification
│   ├── sync/
│   │   ├── sync.go              # Cloud sync client
│   │   └── backup.go            # Backup operations
│   ├── ui/
│   │   ├── model.go             # Central UI model
│   │   ├── messages/
│   │   │   └── messages.go      # Tea messages
│   │   ├── styles/
│   │   │   ├── styles.go        # Style definitions
│   │   │   └── themes.go        # Theme management
│   │   └── views/
│   │       ├── main_view.go     # Main host list view
│   │       ├── edit.go          # Host edit view
│   │       ├── transfer.go      # File transfer view
│   │       ├── initial_prompt.go # Startup prompts
│   │       ├── password_manager.go
│   │       └── key_manager.go
│   └── utils/
│       └── paths.go             # Path utilities
├── screenshots/
├── build.sh
├── go.mod
└── README.md
```

## Core Components

### Entry Point

**File**: `cmd/sshm/main.go`

The entry point initializes the Bubble Tea program:
- Creates the initial `programModel`
- Enters the Elm-like Update-View loop
- Handles SSH session lifecycle (releasing and restoring terminal control)
- Manages application restart for SSH sessions

**Key responsibilities**:
- Initialize cipher from master password
- Load or prompt for API key
- Handle SSH session terminal handoff
- Manage application state transitions

### Configuration Management

**Package**: `internal/config`

Handles loading, saving, and managing the application configuration.

**Key features**:
- JSON serialization/deserialization
- CRUD operations for hosts, passwords, and keys
- Automatic cloud synchronization on save
- Key storage with proper file permissions
- API key management

**Files**:
- **config.go**: Main config manager
- **models/config.go**: Config data structure
- **models/host.go**: Host data structure
- **models/password.go**: Password data structure
- **models/key.go**: SSH key data structure

### Encryption System

**Package**: `internal/crypto`

Provides AES-256-GCM encryption and decryption for sensitive data.

**Algorithm**: AES-256 in GCM (Galois/Counter Mode) for authenticated encryption

**Process**:
1. Derive 32-byte key from master password using PBKDF2-like derivation (or direct hashing)
2. Generate random 12-byte nonce for each encryption
3. Encrypt plaintext and compute authentication tag
4. Store nonce + ciphertext + tag as base64-encoded string

**Key responsibilities**:
- Encrypt/decrypt passwords
- Encrypt/decrypt private keys
- Encrypt/decrypt API key
- Secure key derivation from master password

### SSH Client

**Package**: `internal/ssh`

Wraps `golang.org/x/crypto/ssh` to provide high-level SSH operations.

**Features**:
- Password and public key authentication
- Known hosts verification and management
- Interactive shell sessions
- SFTP file browsing and transfers
- SCP file operations
- Keep-alive for long-running sessions

**Files**:
- **client.go**: SSH client wrapper
- **session.go**: Interactive terminal session handling
- **sftp.go**: SFTP operations (list, read, write, delete)
- **known_hosts.go**: Host key verification

**Session Management**:
1. Connect to host with credentials
2. Configure terminal (xterm-256color, PTY size)
3. Start interactive shell
4. Forward terminal I/O (stdin, stdout, stderr)
5. Handle terminal resize signals
6. Clean up on exit

### UI Framework

**Package**: `internal/ui`

Built on Bubble Tea, following the Elm architecture (Model-Update-View).

**Core Model** (`ui/model.go`):
- Central state container
- Configuration reference
- Active view tracking
- SSH client lifecycle
- Theming and styles

**Views** (`ui/views/`):
- **main_view.go**: Host list and navigation
- **edit.go**: Host/password/key editing forms
- **transfer.go**: Dual-pane file transfer
- **initial_prompt.go**: Master password and API key prompts
- **password_manager.go**: Password CRUD
- **key_manager.go**: SSH key CRUD

**Message Handling**:
- `PasswordEnteredMsg`: Master password entered
- `ApiKeyEnteredMsg`: API key or local mode selected
- `ReloadAppMsg`: Request application restart
- Standard Bubble Tea messages (KeyMsg, MouseMsg, WindowSizeMsg)

**Styles and Themes** (`ui/styles/`):
- Multiple color schemes
- Lip Gloss style definitions
- Dynamic theme switching

**Mouse Support**:
- Click selection
- Double-click actions (connect, navigate)
- Scroll wheel support
- Accurate click position calculations

### Cloud Synchronization

**Package**: `internal/sync`

Handles synchronization with the sshm.io API.

**Features**:
- Push local configuration to cloud
- Pull remote configuration from cloud
- Conflict resolution (latest wins)
- Automatic backups before sync
- Restore from backup on failure

**API Endpoints**:
- `POST /api/sync`: Push encrypted data
- `GET /api/sync`: Pull encrypted data

**Security**:
- Data is encrypted before transmission
- API key is encrypted at rest
- TLS for transport security

## Data Flow

### Startup Flow

```
User launches application
    ↓
Load existing config path
    ↓
Prompt for master password
    ↓
Derive encryption key
    ↓
Check for encrypted API key
    ↓
┌─────────────┐
│ API key?    │
└──┬──────┬───┘
   │Yes   │No
   ↓      ↓
 Sync  Prompt (ESC = local mode)
   ↓      ↓
 Load main view
   ↓
Display host list
```

### Host Connection Flow

```
User selects host and presses Enter
    ↓
Retrieve host credentials
    ↓
Decrypt password/key
    ↓
Establish SSH connection
    ↓
Verify host key (known_hosts)
    ↓
Authenticate (password or key)
    ↓
Release terminal to SSH session
    ↓
User interacts with remote shell
    ↓
Session ends (user exits or error)
    ↓
Restore terminal to TUI
    ↓
Display session ended popup
```

### File Transfer Flow

```
User enters transfer mode
    ↓
Connect to host via SFTP
    ↓
Display dual-pane interface
    ↓
User navigates and selects files
    ↓
User initiates copy (F5)
    ↓
Determine direction (local → remote or vice versa)
    ↓
Transfer file(s) with progress indication
    ↓
Update panel display
```

### Configuration Save Flow

```
User modifies host/password/key
    ↓
Encrypt sensitive fields
    ↓
Update in-memory config
    ↓
Serialize to JSON
    ↓
Write to file (atomic)
    ↓
┌─────────────────┐
│ Cloud sync on?  │
└──┬──────────┬───┘
   │Yes       │No
   ↓          ↓
Push to API  Done
   ↓
Done
```

## State Management

### UI State

The UI model (`ui.Model`) is the single source of truth for UI state:
- **ActiveView**: Current view (main, edit, transfer)
- **HostIndex**: Selected host
- **EditMode**: Whether editing host, password, or key
- **Config**: Reference to configuration manager
- **SSHClient**: Active SSH client (nil if disconnected)
- **Theme**: Current theme index
- **LocalMode**: Whether cloud sync is disabled
- **Quitting**: Application exit flag

State updates are handled via the Bubble Tea message loop:
1. User input generates a message (KeyMsg, MouseMsg)
2. Current view's `Update()` method processes the message
3. State changes are applied to the model
4. `View()` method re-renders based on new state

### Persistence State

Configuration state is persisted to disk:
- Hosts, passwords, keys stored in `ssh_hosts.json`
- API key stored separately in `api_key.txt`
- Private keys stored in `keys/` directory
- All files encrypted with master password-derived key

## Security Architecture

### Defense in Depth

1. **Encryption at Rest**: All sensitive data encrypted with AES-256-GCM
2. **File Permissions**: Config directory and key files use restrictive permissions (0600)
3. **Memory Handling**: Sensitive data cleared from memory when no longer needed (Go GC)
4. **Host Key Verification**: SSH host keys verified via known_hosts
5. **Authenticated Encryption**: GCM mode provides both confidentiality and integrity
6. **Key Derivation**: Master password stretched using PBKDF2 or similar

### Threat Model

**Protected Against**:
- Unauthorized file access (encryption at rest)
- Man-in-the-middle attacks (SSH host key verification)
- Data tampering (GCM authentication tag)
- Credential reuse attacks (unique nonces for each encryption)

**Not Protected Against**:
- Memory dumps while application is running (encrypted data is decrypted in memory)
- Keyloggers or screen capture (user input visible)
- Weak master passwords (user responsibility)
- Compromised remote SSH servers (standard SSH risks)

### Cryptographic Details

- **Cipher**: AES-256-GCM
- **Key Derivation**: SHA-256 hash of master password (or PBKDF2 in enhanced versions)
- **Nonce**: 12 bytes, randomly generated per encryption
- **Tag Size**: 16 bytes (128 bits)
- **Encoding**: Base64 for storage

## File System Layout

```
~/.config/sshm/
├── ssh_hosts.json         # Encrypted config
│   {
│     "hosts": [...],
│     "passwords": [...],
│     "keys": [...]
│   }
├── api_key.txt            # Encrypted API key
├── keys/                  # Encrypted private keys
│   ├── key_0.pem
│   ├── key_1.pem
│   └── ...
└── backups/               # Auto-generated backups
    ├── backup_20240101_120000/
    │   ├── ssh_hosts.json
    │   └── keys/
    └── ...
```

## Extension Points

The architecture is designed to be extensible:

### Adding New Views
1. Create a new view struct implementing `tea.Model`
2. Register the view in `ui/model.go`
3. Add navigation logic in parent views

### Adding New Authentication Methods
1. Extend the `models.Host` structure
2. Update SSH client to handle new method
3. Update edit view to present new options

### Adding New Cloud Providers
1. Implement the sync interface
2. Add provider selection logic
3. Update API key prompt to support multiple providers

### Custom Themes
1. Define a new theme in `ui/styles/themes.go`
2. Add theme selection logic
3. (Future) Support loading themes from config file

## Performance Considerations

- **Lazy Loading**: Configuration loaded once at startup
- **Caching**: In-memory host/password/key lists avoid repeated decryption
- **Efficient Rendering**: Bubble Tea uses ANSI escape codes for minimal redraws
- **Async I/O**: File transfers and SSH operations don't block the UI
- **Minimal Dependencies**: Small binary size (~10 MB) and low memory footprint

## Design Patterns

### Model-View-Update (Elm Architecture)
- Clean separation of concerns
- Predictable state management
- Testable components

### Factory Pattern
- View creation (NewMainView, NewEditView, etc.)
- Client creation (NewSSHClient)

### Strategy Pattern
- Authentication methods (password vs. key)
- Sync backends (local vs. cloud)

### Singleton
- Configuration manager (one instance per application)
- SSH client (one active session at a time)

### Observer (via Messages)
- UI components communicate via messages
- Decoupled event handling

---

[Back to Main Documentation](../README.md)
