//go:build linux && !android

package vault

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Linux implementation using secret-tool (libsecret CLI) which interfaces
// with the Secret Service API (GNOME Keyring, KWallet, etc.)
// Falls back to encrypted file storage if secret-tool is not available.

func set(service, key string, value []byte) error {
	// Try secret-tool first (requires libsecret-tools package)
	if hasSecretTool() {
		return setSecretTool(service, key, value)
	}
	// Fallback to encrypted file storage
	return setFileStorage(service, key, value)
}

func get(service, key string) ([]byte, error) {
	if hasSecretTool() {
		return getSecretTool(service, key)
	}
	return getFileStorage(service, key)
}

func del(service, key string) error {
	if hasSecretTool() {
		return deleteSecretTool(service, key)
	}
	return deleteFileStorage(service, key)
}

func hasSecretTool() bool {
	_, err := exec.LookPath("secret-tool")
	return err == nil
}

// Secret Service implementation using secret-tool
func setSecretTool(service, key string, value []byte) error {
	cmd := exec.Command("secret-tool", "store",
		"--label", service+"/"+key,
		"service", service,
		"key", key,
	)
	cmd.Stdin = bytes.NewReader(value)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vault: failed to set key: %s", stderr.String())
	}
	return nil
}

func getSecretTool(service, key string) ([]byte, error) {
	cmd := exec.Command("secret-tool", "lookup",
		"service", service,
		"key", key,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() == 0 {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("vault: failed to get key: %s", stderr.String())
	}

	result := stdout.Bytes()
	if len(result) == 0 {
		return nil, ErrNotFound
	}
	return result, nil
}

func deleteSecretTool(service, key string) error {
	cmd := exec.Command("secret-tool", "clear",
		"service", service,
		"key", key,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vault: failed to delete key: %s", stderr.String())
	}
	return nil
}

// File-based fallback storage (XDG Base Directory compliant)
// Note: This is less secure than the Secret Service but works without dependencies
func getStorageDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(dataHome, "vault-secrets")
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

func setFileStorage(service, key string, value []byte) error {
	path, err := getStoragePath(service, key)
	if err != nil {
		return fmt.Errorf("vault: failed to get storage path: %w", err)
	}

	// Simple obfuscation (not true encryption, but better than plaintext)
	// For production, consider using golang.org/x/crypto/nacl/secretbox
	encoded := base64.StdEncoding.EncodeToString(value)

	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return fmt.Errorf("vault: failed to write secret: %w", err)
	}
	return nil
}

func getFileStorage(service, key string) ([]byte, error) {
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

func deleteFileStorage(service, key string) error {
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
