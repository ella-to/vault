
```
██╗░░░██╗░█████╗░██╗░░░██╗██╗░░░░░████████╗
██║░░░██║██╔══██╗██║░░░██║██║░░░░░╚══██╔══╝
╚██╗░██╔╝███████║██║░░░██║██║░░░░░░░░██║░░░
░╚████╔╝░██╔══██║██║░░░██║██║░░░░░░░░██║░░░
░░╚██╔╝░░██║░░██║╚██████╔╝███████╗░░░██║░░░
░░░╚═╝░░░╚═╝░░╚═╝░╚═════╝░╚══════╝░░░╚═╝░░░
```

<div align="center">

[![Go Reference](https://pkg.go.dev/badge/ella.to/vault.svg)](https://pkg.go.dev/ella.to/vault)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A simple, cross-platform secret storage library for Go. Stores secrets securely using platform-native secure storage mechanisms without CGO.

</div>

## Features

- **Simple API**: Just `Set`, `Get`, and `Delete`
- **Cross-platform**: macOS, Windows, Linux, iOS, Android, and Browser (wasm)
- **No CGO**: Pure Go implementation using platform CLI tools and file-based fallbacks
- **Binary-safe**: Handles binary data and special characters

## Installation

```bash
go get ella.to/vault@v0.0.3
```

## Usage

```go
package main

import (
    "fmt"
    "log"

    "ella.to/vault"
)

func main() {
    service := "myapp"
    key := "api-key"
    secret := []byte("super-secret-value")

    // Store a secret
    if err := vault.Set(service, key, secret); err != nil {
        log.Fatal(err)
    }

    // Retrieve a secret
    value, err := vault.Get(service, key)
    if err != nil {
        if err == vault.ErrNotFound {
            fmt.Println("Secret not found")
        } else {
            log.Fatal(err)
        }
    }
    fmt.Printf("Secret: %s\n", value)

    // Delete a secret
    if err := vault.Del(service, key); err != nil {
        log.Fatal(err)
    }
}
```

## Platform Implementations

| Platform | Storage Mechanism | Notes |
|----------|-------------------|-------|
| **macOS** | Keychain via `security` CLI | Uses the default keychain |
| **Windows** | Credential Manager via PowerShell (advapi32 `CredRead`/`CredWrite`/`CredDelete`) | Generic credentials |
| **Linux** | Secret Service via `secret-tool` | Falls back to encrypted files if unavailable |
| **iOS** | File-based in app sandbox | Uses iOS Data Protection |
| **Android** | File-based in app sandbox | Requires `HOME` to be set (see notes) |
| **WASM/Browser** | IndexedDB | Base64 encoded, same-origin accessible |

### Platform Notes

#### macOS
Uses the `security` command-line tool to interact with the Keychain. No additional setup required.

#### Windows
Uses PowerShell to call the Credential Manager APIs (`CredRead`/`CredWrite`/`CredDelete` in advapi32) directly, so behavior does not depend on localized command output. Works on Windows 10/11.

Note: Generic credential blobs are capped at 2560 bytes by Windows (`CRED_MAX_CREDENTIAL_BLOB_SIZE`). Values are base64 encoded and stored as UTF-16, so secrets up to roughly 960 raw bytes fit.

#### Linux
Prefers `secret-tool` (from `libsecret-tools` package) which integrates with GNOME Keyring, KWallet, etc. If not available, falls back to file-based storage in `~/.local/share/vault-secrets/`.

#### WebAssembly (Browser)
Uses IndexedDB for persistent storage. **Security considerations:**
- Data is accessible to any JavaScript on the same origin
- No hardware-backed encryption (unlike native keychains)
- Data is cleared when user clears browser data
- For sensitive credentials, consider server-side storage or Web Crypto API encryption

Install secret-tool:
```bash
# Debian/Ubuntu
sudo apt install libsecret-tools

# Fedora
sudo dnf install libsecret

# Arch Linux
sudo pacman -S libsecret
```

#### iOS & Android
Use file-based storage within the app's sandboxed storage, which provides OS-level security isolation.

On iOS, `HOME` points at the app container automatically, so secrets land in `Library/Application Support/vault-secrets`.

On Android, app processes do not get a usable `HOME` by default. The embedding app must set it to its private files directory before using this package, e.g. from Java/Kotlin before initializing the Go runtime:

```java
android.system.Os.setenv("HOME", context.getFilesDir().getAbsolutePath(), true);
```

## Testing

### Run tests on current platform
```bash
go test -v ./...
# or
make test
```

### Cross-compilation verification
Verify the code compiles for all platforms:
```bash
make build-all
```

### Testing on Linux (via Docker)
```bash
make test-linux
# or manually:
docker run --rm -v $(pwd):/app -w /app golang:latest go test -v ./...
```

### Testing WebAssembly (Node.js)
Runs the full test suite as `js/wasm` under Node.js with the [`fake-indexeddb`](https://github.com/dumbmatter/fakeIndexedDB) polyfill (requires `node` and `npm`):
```bash
make test-wasm
```

### Testing on Windows
Tests run on `windows-latest` in the GitHub Actions workflow at `.github/workflows/test.yml`, which covers Linux, macOS, Windows, the Linux keyring path (gnome-keyring), js/wasm, and cross-compilation of every target. Locally, a Windows VM (VirtualBox, Parallels, VMware) also works.

### Testing on mobile platforms
- **iOS**: Use Xcode simulator or device via gomobile
- **Android**: Use Android emulator or device via gomobile

## API Reference

### Functions

#### `Set(service, key string, value []byte) error`
Stores a secret. Overwrites if it already exists.

#### `Get(service, key string) ([]byte, error)`
Retrieves a secret. Returns `ErrNotFound` if not found.

#### `Del(service, key string) error`
Deletes a secret. Returns `ErrNotFound` if not found.

### Errors

- `ErrNotFound`: The requested key does not exist
- `ErrInvalidKey`: Service or key is empty
- `ErrInvalidValue`: Value is empty or nil

## Security Considerations

1. **macOS/Windows**: Secrets are stored in platform-native secure storage with OS-level encryption
2. **Linux**: With `secret-tool`, uses the system keyring. File fallback uses base64 encoding (not encrypted)
3. **iOS/Android**: File-based storage relies on OS sandbox isolation
4. **Memory**: Secrets are held in memory as `[]byte`; consider zeroing after use for sensitive data

## License

MIT
