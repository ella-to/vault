//go:build android

package vault

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Android implementation using file-based storage in the app's private directory.
// On Android, the app's internal storage (/data/data/<package>/) is private
// and only accessible by the app itself.
//
// Note: For true Android Keystore access, CGO with JNI is required.
// This implementation provides a secure fallback using Android's app sandbox.

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
	// Store under the app's private files directory. Android does not set
	// HOME for app processes by default; the embedding app must set it to
	// its sandbox (e.g. Context.getFilesDir().getAbsolutePath()) before
	// using this package. ANDROID_DATA is deliberately not used: it points
	// at /data, which apps have no permission to write to.
	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		dir := filepath.Join(home, ".vault-secrets")
		return dir, os.MkdirAll(dir, 0o700)
	}

	// Fallback for environments that run with a writable working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cwd, ".vault-secrets")
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
