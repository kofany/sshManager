# SSH Manager

SSH Manager is a terminal-based application for managing SSH connections, file transfers, and credentials with an intuitive user interface and cloud synchronization capabilities.

---

## Features

- Secure storage of SSH credentials and keys
- File transfer capabilities (SFTP/SCP)
- Cloud synchronization (optional)
- Multiple color themes
- Interactive terminal sessions
- Local and remote file browsing
- Password and SSH key authentication

---

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/sshmanager.git

# Build the application
cd sshmanager
go build -o sshm
```

---

## Usage

### Basic Navigation

- `↑/↓` or `k/j` - Navigate through lists
- `Tab` - Switch between panels
- `ESC` - Go back/Cancel
- `q` - Quit application
- `Space` - Switch color theme

---

### Host Management

- `h` - Add new host
- `e` or `F4` - Edit selected host
- `d` or `F8` - Delete selected host
- `c` or `Enter` - Connect to selected host

---

### Password Management

- `p` - Open password management
- `a` - Add new password
- `e` - Edit selected password
- `d` - Delete selected password

---

### SSH Key Management

- `k` - Open SSH key management
- `a` - Add new SSH key
- `e` - Edit selected key
- `d` - Delete selected key

---

### File Transfer Mode

- `t` - Enter file transfer mode when host is selected
- `Tab` - Switch between local and remote panels
- `F5` or `c` - Copy file/directory
- `F6` or `r` - Rename file/directory
- `F7` or `m` - Create new directory
- `F8` or `d` - Delete file/directory
- `s` - Select/deselect item for batch operations
- `Enter` - Enter directory

Additionally, for function key operations like in Midnight Commander:
- `ESC + [number]` also triggers the corresponding function key (e.g., `ESC + 5` for `F5`).

---

### Terminal Session

- All standard terminal shortcuts work in SSH sessions
- Session automatically handles terminal resize
- Keep-alive functionality to maintain connection

---

## Configuration

The application stores its configuration in:

- **Linux/Mac:** `~/.config/sshm/ssh_hosts.json`
- **Windows:** `%USERPROFILE%\.config\sshm\ssh_hosts.json`

---

## Cloud Synchronization

1. Register at [https://sshm.io](https://sshm.io) to get an API key
2. Enter the API key when prompted on first run
3. Press `ESC` to work in local mode without synchronization

---

## Security Features

- AES-256-GCM encryption for sensitive data
- Secure storage of passwords and private keys
- Automatic backup before sync operations
- Support for SSH key authentication

---

## Keyboard Shortcuts Reference

### Main View

- **Connect to host:** `c/Enter`
- **Add new host:** `h`
- **Edit host:** `e/F4`
- **Delete host:** `d/F8`
- **Password management:** `p`
- **SSH key management:** `k`
- **File transfer mode:** `t`
- **Switch theme:** `Space`
- **Quit:** `q/Ctrl+c`

### File Transfer Mode

- **Switch panels:** `Tab`
- **Copy:** `F5/c`
- **Rename:** `F6/r`
- **Make directory:** `F7/m`
- **Delete:** `F8/d`
- **Select item:** `s`
- **Open directory:** `Enter`
- **Return to main view:** `q`

---

## Troubleshooting

### Common Issues

#### Connection Issues

- Verify host information
- Check network connectivity
- Ensure correct credentials

#### Sync Problems

- Verify API key
- Check internet connection
- Ensure backup is available

#### Permission Issues

- Check SSH key permissions (should be 600)
- Verify user permissions on remote host

---

## Development

The application is built using:

- Go programming language
- Bubble Tea TUI framework
- Lip Gloss styling
- Crypto/SSH for SSH functionality
- SFTP/SCP for file transfers

---

## Support

For support, please:

- Report issues on GitHub
- Contact support at [j@dabrowski.biz](mailto:j@dabrowski.biz)

---

## License

This project is licensed under the GNU GPL v3 License - see the LICENSE file for details.
