# Security Guide

This document outlines the security model of SSH Manager (sshm) and provides best practices for secure operation.

## Table of Contents

- [Security Overview](#security-overview)
- [Threat Model](#threat-model)
- [Encryption Details](#encryption-details)
- [Credential Storage](#credential-storage)
- [Network Security](#network-security)
- [Cloud Synchronization Security](#cloud-synchronization-security)
- [Operational Security Best Practices](#operational-security-best-practices)
- [Compliance Considerations](#compliance-considerations)
- [Security Auditing](#security-auditing)
- [Incident Response](#incident-response)

## Security Overview

SSH Manager is designed with security as a first-class consideration. The primary security principles are:

- **Confidentiality**: All sensitive data is encrypted at rest
- **Integrity**: Authenticated encryption ensures data is not tampered with
- **Availability**: Local backups and offline mode maintain functionality
- **Least Privilege**: Minimal permissions for stored files and directories
- **Transparency**: Open-source code enables independent verification

## Threat Model

### Protected Against
- Compromise of local filesystem storage (due to encryption at rest)
- Accidental disclosure of credentials (encrypted files)
- MITM attacks during SSH connections (host key verification)
- Unauthorized API access (encrypted API keys)
- Data tampering (GCM authentication tags)

### Not Protected Against
- Active malware/keyloggers on the client machine
- Memory scraping attacks while the application is running
- Compromised remote SSH servers (beyond scope)
- Weak or reused master passwords
- Physical access to unlocked, running session

## Encryption Details

| Component | Algorithm | Details |
|-----------|-----------|---------|
| Master Password | SHA-256 | Derives a 32-byte key for AES encryption |
| Data Encryption | AES-256-GCM | Authenticated encryption, 12-byte nonce, 16-byte tag |
| API Key Encryption | AES-256-GCM | Same mechanism as data encryption |
| SSH | RSA/Ed25519/etc. | Dependent on server configuration |

### Encryption Workflow
1. When the master password is entered, a 32-byte key is derived
2. A random 12-byte nonce is generated for each encryption operation
3. Plaintext (password, key, API key) is encrypted with AES-256-GCM
4. Ciphertext, nonce, and authentication tag are stored together
5. Data is base64-encoded before being written to disk

## Credential Storage

### Passwords
- Stored in the configuration file (`ssh_hosts.json`) as encrypted strings
- Associated with hosts by index reference (no duplication)
- Decrypted only when needed for SSH authentication

### SSH Keys
- Imported keys stored in `~/.config/sshm/keys/`
- Files saved with `0600` permissions
- File paths stored in the encrypted configuration
- Key data can either be imported or referenced externally

### API Key
- Stored in `~/.config/sshm/api_key.txt`
- Fully encrypted using AES-256-GCM
- Decrypted only when synchronizing

## Network Security

### SSH Connections
- Uses `golang.org/x/crypto/ssh`
- Supports password and public key authentication
- Host key verification against known_hosts file
- Supports modern ciphers and algorithms configured on server

### SFTP and SCP
- Operate over SSH tunnel
- SFTP for structured file operations
- SCP for file copy operations
- Both inherit SSH security characteristics

### Cloud Sync Communications
- HTTPS/TLS for transport security
- Encrypted payloads (double protection)
- API key required for authentication
- Automatic retries with exponential backoff (configurable)

## Cloud Synchronization Security

### Data Handling
- All data remains encrypted before leaving the client
- Server never sees plaintext credentials
- AES-256-GCM ensures integrity during transit

### API Key Protection
- API key is encrypted with master password-derived key
- No plaintext key stored on disk
- Key only decrypted in-memory during synchronization

### Backups
- Before synchronization, backups are created locally
- Backups stored in `~/.config/sshm/backups/`
- Backups contain encrypted data (same as live configuration)

### Local Mode
- For maximum security, users can operate in local-only mode
- Press `ESC` at API prompt to disable synchronization
- No outbound data is transmitted in local mode

## Operational Security Best Practices

1. **Strong Master Password**
   - Use a unique, randomly generated passphrase
   - Consider using a password manager to store the master password

2. **File Permissions**
   ```bash
   chmod 700 ~/.config/sshm
   chmod 600 ~/.config/sshm/ssh_hosts.json
   chmod 600 ~/.config/sshm/api_key.txt
   chmod 600 ~/.config/sshm/keys/*
   ```

3. **Host Key Verification**
   - Always verify host keys when connecting to new servers
   - Maintain `known_hosts` file
   - Use SSH fingerprints to confirm authenticity

4. **Regular Updates**
   - Keep the application updated to receive security patches
   - Monitor project releases for security advisories

5. **Network Hygiene**
   - Use VPN when connecting over untrusted networks
   - Restrict SSH access to specific IP addresses when possible

6. **Disable Unused Features**
   - If not using cloud sync, remain in local mode
   - Remove saved credentials that are no longer needed

7. **Environment Isolation**
   - Run application on a trusted machine
   - Ensure system is free from malware

## Compliance Considerations

- **GDPR/PII**: Credentials may contain personal data; ensure compliance
- **PCI DSS**: If storing payment-related SSH credentials, secure storage meets basic requirements
- **HIPAA**: Encryption and access controls align with HIPAA requirements (implementation responsibility)
- **Internal Policies**: Ensure master password is managed according to organization policy

## Security Auditing

### Audit Checklist
- [ ] Master password meets complexity requirements
- [ ] Config directory permissions are correct
- [ ] SSH keys stored securely
- [ ] API key rotations are performed regularly
- [ ] Backups stored securely in separate location
- [ ] Cloud sync logs reviewed (where applicable)

### Logging
- Local logging is minimal to avoid storing sensitive data
- Critical events (sync success/failure) displayed in UI
- Consider external logging for audit trails (custom implementation)

### Penetration Testing
- Focus on client environment security
- Ensure no plaintext secrets leak during operation
- Test SSH configurations for vulnerabilities

## Incident Response

### Lost Master Password
- Encrypted data cannot be recovered without master password
- Restore from backup if available
- Reinitialize configuration and re-enter credentials manually

### Suspected Compromise
1. Disconnect from networks
2. Change master password (requires re-encryption of data)
3. Rotate SSH passwords and keys
4. Revoke API key and generate a new one
5. Review system for traces of compromise

### Configuration Breach
- If configuration files are copied but still encrypted:
  - Still a security incident, but exploitation requires master password
  - Consider rotating passwords for defense-in-depth

### Device Loss
- Encrypted data provides protection if attacker lacks master password
- Change passwords for all stored hosts as precaution
- Rotate API key if cloud sync enabled

## Future Security Enhancements

- Configurable key derivation iterations (PBKDF2)
- Support for hardware security modules (HSMs)
- SSH agent integration for ephemeral credentials
- Multi-user support with individual encryption keys
- Audit log export for SIEM integration

---

[Back to Main Documentation](../README.md)
