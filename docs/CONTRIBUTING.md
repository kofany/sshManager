# Contributing to SSH Manager

Thank you for your interest in contributing to SSH Manager (sshm)! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Features](#suggesting-features)
  - [Code Contributions](#code-contributions)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Testing](#testing)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)

## Code of Conduct

This project adheres to a code of conduct that all contributors are expected to follow:

- **Be respectful**: Treat all contributors with respect and dignity
- **Be inclusive**: Welcome contributors of all backgrounds and experience levels
- **Be constructive**: Provide helpful, actionable feedback
- **Be patient**: Remember that contributors are often volunteering their time
- **No harassment**: Harassment, discrimination, or abuse will not be tolerated

## How Can I Contribute?

### Reporting Bugs

Bugs are tracked as [GitHub Issues](https://github.com/kofany/sshmanager/issues).

**Before submitting a bug report**:
- Check existing issues to avoid duplicates
- Try to reproduce with the latest version
- Gather relevant details about your environment

**When reporting a bug, include**:
- **Title**: Clear, descriptive summary
- **Description**: Detailed explanation of the issue
- **Steps to Reproduce**: Exact steps to trigger the bug
- **Expected Behavior**: What should happen
- **Actual Behavior**: What actually happens
- **Environment**: OS, terminal, Go version, sshm version
- **Logs/Screenshots**: Any relevant output or visual artifacts

**Example bug report**:
```markdown
## Description
When trying to connect to a host with SSH key authentication, the application crashes.

## Steps to Reproduce
1. Add a new host with SSH key authentication
2. Select the host and press Enter to connect
3. Application exits with error

## Expected Behavior
Should connect successfully or display authentication error.

## Actual Behavior
Application terminates with panic.

## Environment
- OS: Ubuntu 22.04
- Terminal: GNOME Terminal 3.44
- Go Version: 1.23.3
- sshm Version: 1.0.0

## Stack Trace
```
panic: runtime error: invalid memory address or nil pointer dereference
[stack trace...]
```
```

### Suggesting Features

Feature requests are welcome! Please submit them as GitHub Issues with the label "enhancement".

**When suggesting a feature, include**:
- **Title**: Clear, concise summary of the feature
- **Problem**: What problem does this feature solve?
- **Proposed Solution**: How should the feature work?
- **Alternatives**: Any alternative solutions considered
- **Use Cases**: Real-world scenarios where this feature would be useful
- **Priority**: Low/Medium/High (your perspective)

### Code Contributions

We welcome pull requests for:
- Bug fixes
- Feature implementations
- Documentation improvements
- Test coverage enhancements
- Performance optimizations
- UI/UX improvements

## Development Setup

### Prerequisites
- Go 1.23 or higher
- Git
- Terminal with UTF-8 and color support
- (Optional) golangci-lint for code linting

### Clone and Setup

```bash
# Fork the repository on GitHub
# Clone your fork
git clone https://github.com/YOUR_USERNAME/sshmanager.git
cd sshmanager

# Add upstream remote
git remote add upstream https://github.com/kofany/sshmanager.git

# Install dependencies
go mod download

# Verify the build
go build -o sshm ./cmd/sshm/main.go
```

### Development Tools

```bash
# Install golangci-lint (optional but recommended)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install gofmt (usually bundled with Go)
# Install goimports (optional)
go install golang.org/x/tools/cmd/goimports@latest
```

## Development Workflow

### Branch Strategy
- `main` branch: Stable, production-ready code
- Feature branches: `feature/feature-name`
- Bugfix branches: `bugfix/issue-number-short-description`
- Hotfix branches: `hotfix/critical-issue-description`

### Making Changes

1. **Create a branch**
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make your changes**
   - Follow code standards (see below)
   - Write clear, concise commit messages
   - Include tests for new functionality

3. **Test your changes**
   ```bash
   go test ./...
   go build -o sshm ./cmd/sshm/main.go
   ./sshm  # Manual testing
   ```

4. **Lint your code**
   ```bash
   golangci-lint run
   gofmt -s -w .
   ```

5. **Commit**
   ```bash
   git add .
   git commit -m "feat: add support for SSH proxy jump"
   ```

6. **Keep your branch updated**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

7. **Push to your fork**
   ```bash
   git push origin feature/my-new-feature
   ```

8. **Open a Pull Request** on GitHub

## Code Standards

### Go Code Guidelines
- Follow [Effective Go](https://golang.org/doc/effective_go) conventions
- Use `gofmt` for formatting
- Use `goimports` for import sorting
- Variable names: camelCase
- Constants: camelCase or SCREAMING_SNAKE_CASE (context-dependent)
- Exported identifiers: Start with uppercase letter

### Code Organization
- Keep functions focused and single-purpose
- Prefer small, composable functions over monolithic ones
- Group related functionality in packages
- Use interfaces to define contracts

### Comments
- Document all exported functions, types, and constants
- Use complete sentences with proper punctuation
- Explain "why" not "what" (the code shows "what")
- Avoid obvious comments

**Example**:
```go
// NewSSHClient creates a new SSH client with the given configuration.
// It verifies the host key against known_hosts and establishes a connection.
// Returns an error if the connection fails or host key verification fails.
func NewSSHClient(config *SSHConfig) (*Client, error) {
    // Implementation
}
```

### Error Handling
- Always check errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return errors rather than panicking (except in truly exceptional cases)
- Use sentinel errors for expected error conditions

### Commit Messages
Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `style:` Code style changes (formatting, no logic change)
- `refactor:` Code refactoring
- `test:` Test additions or modifications
- `chore:` Build process or tooling changes

**Example**:
```
feat: add support for SSH proxy jump hosts

- Implement ProxyJump configuration option
- Update SSH client to support proxy connections
- Add UI fields for proxy configuration

Closes #123
```

## Testing

### Unit Tests
```bash
go test ./internal/config
go test ./internal/crypto
go test ./internal/ssh
```

### Integration Tests
```bash
go test -tags=integration ./...
```

### Manual Testing
- Test on different platforms (Linux, macOS, Windows)
- Test in different terminals
- Test mouse interactions
- Test keyboard navigation
- Test edge cases (empty lists, long values, special characters)

## Documentation

### Code Documentation
- Document all exported functions and types
- Use GoDoc-style comments
- Include examples when helpful

### User Documentation
- Update README.md for major features
- Update docs/ for detailed guides
- Update CHANGELOG.md for each release

### Architecture Documentation
- Update ARCHITECTURE.md when adding major components
- Document design decisions

## Pull Request Process

### Before Submitting
- [ ] Code is formatted with `gofmt`
- [ ] All tests pass
- [ ] Linter checks pass
- [ ] Commit messages follow conventions
- [ ] Documentation is updated
- [ ] Branch is rebased on latest `main`

### PR Description
Include:
- Summary of changes
- Related issue numbers (e.g., "Closes #123")
- Testing performed
- Screenshots/videos (for UI changes)
- Breaking changes (if any)

### Review Process
- Maintainers will review your PR
- Address feedback by pushing new commits
- Once approved, the PR will be merged

### After Merge
- Delete your branch
- Pull latest `main` for next contribution

## Release Process

Releases are managed by maintainers:

1. **Version Bump**: Update version in `go.mod` or appropriate location
2. **Changelog**: Update CHANGELOG.md with changes since last release
3. **Tag**: Create a Git tag following semantic versioning (e.g., `v1.2.0`)
4. **Build**: Build binaries for all platforms using `build.sh`
5. **Release**: Create a GitHub Release with binaries and changelog
6. **Announce**: Notify users via appropriate channels

## Questions?

If you have questions, feel free to:
- Open an issue with the "question" label
- Email the maintainers at [j@dabrowski.biz](mailto:j@dabrowski.biz)
- Check the [FAQ](USER_GUIDE.md#faq) in the User Guide

Thank you for contributing to SSH Manager!

---

[Back to Main Documentation](../README.md)
