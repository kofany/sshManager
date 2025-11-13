# User Guide

Welcome to SSH Manager (sshm). This guide covers everything you need to know to get the most out of the application, from initial setup to advanced functionality.

## Table of Contents

- [Getting Started](#getting-started)
  - [First Run Experience](#first-run-experience)
  - [User Interface Overview](#user-interface-overview)
- [Main View](#main-view)
  - [Creating a Host](#creating-a-host)
  - [Connecting to a Host](#connecting-to-a-host)
  - [Editing Hosts](#editing-hosts)
  - [Deleting Hosts](#deleting-hosts)
  - [Sorting and Filtering](#sorting-and-filtering)
- [File Transfer Mode](#file-transfer-mode)
  - [Dual-Pane Layout](#dual-pane-layout)
  - [Transferring Files](#transferring-files)
  - [Batch Operations](#batch-operations)
  - [Renaming and Deleting](#renaming-and-deleting)
  - [Working with Directories](#working-with-directories)
- [Password Management](#password-management)
  - [Password Storage](#password-storage)
  - [Adding Passwords](#adding-passwords)
  - [Linking Passwords to Hosts](#linking-passwords-to-hosts)
- [SSH Key Management](#ssh-key-management)
  - [Key Storage](#key-storage)
  - [Adding SSH Keys](#adding-ssh-keys)
  - [Using Existing Key Files](#using-existing-key-files)
  - [Linking Keys to Hosts](#linking-keys-to-hosts)
- [Terminal Sessions](#terminal-sessions)
  - [Interactive Shell](#interactive-shell)
  - [Session Keep-Alive](#session-keep-alive)
  - [Releasing the Terminal](#releasing-the-terminal)
- [Cloud Synchronization](#cloud-synchronization)
  - [Setting Up Synchronization](#setting-up-synchronization)
  - [Manual Sync Controls](#manual-sync-controls)
  - [Working Offline](#working-offline)
- [Keyboard Shortcuts](#keyboard-shortcuts)
- [Mouse Interaction](#mouse-interaction)
- [Themes and Appearance](#themes-and-appearance)
- [Backup and Restore](#backup-and-restore)
- [Advanced Tips](#advanced-tips)
- [FAQ](#faq)

## Getting Started

### First Run Experience

When you launch SSH Manager for the first time:

1. **Master Password**
   - You'll be prompted to create a master password
   - This password is used to derive encryption keys for all sensitive data
   - Choose a strong and memorable password
   - You must enter this password each time you start the application

2. **Cloud Synchronization Prompt**
   - The application checks for an existing encrypted API key
   - If none is found, you can enter an API key for [sshm.io](https://sshm.io)
   - Press `ESC` to skip and work in local-only mode
   - You can enable sync later from the configuration menu

3. **Initial Main View**
   - The main interface loads
   - A default host list is displayed (empty on first run)
   - Status bar shows current mode, connection status, and hints

### User Interface Overview

The user interface is divided into distinct areas:

- **Title Bar**: Displays application name and active mode
- **Host List Panel**: Shows saved SSH hosts
- **Detail Panel**: Displays selected host details or contextual information
- **Status Bar**: Shows mode, hints, and notifications
- **Pop-up Dialogs**: Used for confirmation, warnings, or detailed input

## Main View

The main view is the central hub for managing your SSH hosts.

### Creating a Host

1. Press `h` (or use the Add Host button via mouse)
2. Fill in the host details:
   - **Name**: Friendly name for the host
   - **Address**: Hostname or IP address
   - **Port**: Default is 22
   - **Username**: SSH username
   - **Authentication method**: Choose password or SSH key
   - **Description**: Optional notes
3. Save the host to encrypt and store it in your configuration

### Connecting to a Host

- Select a host using arrow keys, mouse, or search
- Press `c` or `Enter`, or double-click the host with the mouse
- If authentication succeeds, an interactive terminal session opens
- Press `Ctrl+C` to exit the remote session when finished

### Editing Hosts

- Select a host and press `e` (or `F4`)
- Modify the necessary fields
- When switching from password to key authentication (or vice versa), the UI prompts for relevant fields
- Save changes to update the encrypted configuration

### Deleting Hosts

- Select a host and press `d` (or `F8`)
- Confirm the deletion when prompted
- The host is removed and the configuration is re-encrypted

### Sorting and Filtering

- Use the search filter to quickly find hosts by name
- Sort hosts by name, last used, or custom order (depending on version)
- Use keyboard shortcuts or UI buttons for sorting controls

## File Transfer Mode

The file transfer mode provides a dual-pane interface for local and remote file management using SFTP/SCP.

### Dual-Pane Layout

- **Left Panel**: Local filesystem
- **Right Panel**: Remote server filesystem
- Active panel is highlighted and receives keyboard input

### Transferring Files

1. Navigate to the file or directory you want to transfer
2. Select the item (it highlights)
3. Press `F5` or `c` to initiate transfer
4. Choose direction if prompted (local \<-> remote)
5. Transfer progress is shown in status area

### Batch Operations

- Press `s` to select multiple items for batch operations
- Use `F5` to transfer or `F8` to delete multiple selected items
- Selected items are marked with a special indicator

### Renaming and Deleting

- `F6` or `r` to rename the selected file/directory
- `F8` or `d` to delete (with confirmation)
- Supports both local and remote operations depending on active panel

### Working with Directories

- Press `Enter` or double-click to enter a directory
- `Backspace` or dedicated hotkey to navigate up one level
- Hidden files may be toggled based on configuration

## Password Management

Access the password management view by pressing `p` in the main view.

### Password Storage

- All passwords are encrypted using AES-256-GCM
- Passwords are stored separately from host data
- Password list displays descriptive names, not plaintext passwords

### Adding Passwords

1. Press `a` to add a new password entry
2. Provide a descriptive name and the password itself
3. Optionally add notes or additional metadata
4. Save to encrypt and store the password

### Linking Passwords to Hosts

- When editing a host, select from the list of stored passwords
- Linking is done by reference (no password duplication)
- A host can only use one password; multiple hosts can share a password entry

## SSH Key Management

Access SSH key management by pressing `k` in the main view.

### Key Storage

- SSH keys can be stored as references to existing files or as imported key material
- Imported keys are stored in the encrypted keys directory (`~/.config/sshm/keys/`)
- Permissions are enforced (600) for stored keys

### Adding SSH Keys

1. Press `a` to add a new key entry
2. Provide a description and key type (RSA, Ed25519, etc.)
3. Paste the private key or choose to reference a file path
4. Optionally add passphrase information (for user reference)
5. Save to encrypt key metadata

### Using Existing Key Files

- Choose "Reference existing key" and provide the file path
- The application will use the provided path without importing
- Useful when key is managed externally (e.g., with ssh-agent)

### Linking Keys to Hosts

- When editing a host, choose the "Use SSH key" authentication method
- Select the desired key from the list
- Optionally, specify passphrase if required (stored encrypted)

## Terminal Sessions

### Interactive Shell

- SSH sessions open in the native terminal
- Full keyboard input is supported (control sequences, etc.)
- Mouse events are passed through when supported by the remote shell

### Session Keep-Alive

- Automatic keep-alive packets can be enabled to maintain long sessions
- Prevents disconnections due to inactivity
- Configurable in advanced settings

### Releasing the Terminal

- The application temporarily releases control of the terminal during SSH sessions
- Upon session exit, control returns to the TUI
- If an error occurs, a pop-up displays diagnostic information

## Cloud Synchronization

### Setting Up Synchronization

1. Register at [sshm.io](https://sshm.io)
2. Generate an API key from the dashboard
3. Enter the API key when prompted by the application
4. The API key is encrypted and stored locally
5. Initial synchronization imports data from the cloud (with backups)

### Manual Sync Controls

- Changes are automatically pushed when configuration is saved
- Manual sync commands are available in advanced options
- Sync status is displayed in the status bar

### Working Offline

- Press `ESC` when prompted for API key to work in local mode
- Cloud synchronization is disabled until API key is provided
- Existing local data remains encrypted and usable offline

## Keyboard Shortcuts

| Action | Shortcut |
|--------|----------|
| Add Host | `h` |
| Edit Host | `e` or `F4` |
| Delete Host | `d` or `F8` |
| Connect | `c` or `Enter` |
| File Transfer Mode | `t` |
| Password Management | `p` |
| SSH Key Management | `k` |
| Switch Theme | Space |
| Quit | `q` or `Ctrl+C` |

### File Transfer Shortcuts

| Action | Shortcut |
|--------|----------|
| Switch Panel | Tab |
| Copy | `F5` or `c` |
| Move/Rename | `F6` or `r` |
| Create Directory | `F7` or `m` |
| Delete | `F8` or `d` |
| Select/Deselect | `s` |
| Enter Directory | Enter |
| Exit File Transfer | `q` |

## Mouse Interaction

- **Single Click**: Select list items, focus input fields
- **Double Click**: Connect to host, enter directories
- **Scroll Wheel**: Scroll through lists and panels
- **Drag**: Not currently supported (Bubble Tea limitation)

Double-click detection has a 500 ms threshold. The application remembers the last clicked item to distinguish double-clicks accurately.

## Themes and Appearance

- Press Space to cycle through available themes
- Themes apply to entire UI (panels, lists, status bar)
- Custom theme support is planned via configuration file
- UI adapts to terminal size changes

## Backup and Restore

### Manual Backup

```bash
# Linux/macOS
cp ~/.config/sshm/ssh_hosts.json ~/Backups/
cp ~/.config/sshm/api_key.txt ~/Backups/
cp -r ~/.config/sshm/keys ~/Backups/
```

### Automatic Backup

- Before cloud synchronization, the application creates backups
- Backups stored in `~/.config/sshm/backups/`
- Naming convention: `backup_YYYYMMDD_HHMMSS`
- Files include configuration and keys

### Restore

```bash
# Replace current config with backup
cp ~/Backups/ssh_hosts.json ~/.config/sshm/
cp ~/Backups/api_key.txt ~/.config/sshm/
cp -r ~/Backups/keys ~/.config/sshm/
```

## Advanced Tips

- Use aliases in your shell for quick launch (`alias sshm="~/bin/sshm"`)
- Combine SSH Manager with tmux or screen for multi-session workflows
- Enable clipboard integration via terminal settings
- Use the search function to quickly locate hosts by name
- Export host list (planned feature) to share with team members

## FAQ

### How is my data protected?
All sensitive data is encrypted using AES-256-GCM with a key derived from your master password.

### Can I sync across devices?
Yes, by using the optional cloud synchronization service at [sshm.io](https://sshm.io).

### What if I forget my master password?
The master password is required to decrypt your data. There is no way to recover encrypted data without it.

### Does it support proxy/jump hosts?
Jump host support is planned. Check the project roadmap for updates.

### Can I import existing SSH config?
Import functionality is under development. Currently, hosts must be added manually.

### How do I report bugs?
Open an issue on [GitHub](https://github.com/kofany/sshmanager/issues) with detailed steps to reproduce.

---

[Back to Main Documentation](../README.md)
