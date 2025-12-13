//go:build ios

package vault

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// iOS implementation using file-based storage in the app's secure container.
// On iOS, the app sandbox provides security, and files in the Documents
// directory are encrypted by the device when locked (Data Protection).
//
// Note: For true Keychain access on iOS, CGO with Security.framework is required.
// This implementation provides a secure fallback using iOS file protection.

func set(service, key string, value []byte) error {
	path, err := getStoragePath(service, key)
	if err != nil {
		return fmt.Errorf("vault: failed to get storage path: %w", err)
	}

	// Encode the value for storage
	encoded := base64.StdEncoding.EncodeToString(value)

	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return fmt.Errorf("vault: failed to write secret: %w", err)
	}
	return nil
}

func get(service, key string) ([]byte, error) {
	path, err := getStoragePath(service, key)
	if err != nil {
		return nil, fmt.Errorf("vault: failed to get storage path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("vault: failed to read secret: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("vault: failed to decode secret: %w", err)
	}
	return decoded, nil
}

func del(service, key string) error {
	path, err := getStoragePath(service, key)
	if err != nil {
		return fmt.Errorf("vault: failed to get storage path: %w", err)
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("vault: failed to delete secret: %w", err)
	}
	return nil
}

func getStorageDir() (string, error) {
	// On iOS, use the app's Library directory for private data
	// The Library/Application Support directory is recommended for app data
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Application Support", "vault-secrets")
	return dir, os.MkdirAll(dir, 0o700)
}

func getStoragePath(service, key string) (string, error) {
	dir, err := getStorageDir()
	if err != nil {
		return "", err
	}
	// Use base64 encoding for safe filenames
	filename := base64.URLEncoding.EncodeToString([]byte(service + "/" + key))
	return filepath.Join(dir, filename), nil
}
