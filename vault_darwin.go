//go:build darwin && !ios

package vault

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

// macOS implementation using the `security` command-line tool
// which interfaces with the Keychain without requiring CGO.
// Values are base64 encoded to handle binary data safely.

func set(service, key string, value []byte) error {
	// Delete existing item first (ignore errors if it doesn't exist)
	_ = del(service, key)

	// Base64 encode the value to safely handle binary data
	encoded := base64.StdEncoding.EncodeToString(value)

	// Add new item to keychain
	cmd := exec.Command("security", "add-generic-password",
		"-a", key, // account name
		"-s", service, // service name
		"-w", encoded, // password (base64 encoded value)
		"-U", // update if exists
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vault: failed to set key: %s", stderr.String())
	}

	return nil
}

func get(service, key string) ([]byte, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-a", key, // account name
		"-s", service, // service name
		"-w", // output only the password
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := stderr.String()
		if strings.Contains(errStr, "could not be found") ||
			strings.Contains(errStr, "SecKeychainSearchCopyNext") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("vault: failed to get key: %s", errStr)
	}

	// Remove trailing newline and decode base64
	result := strings.TrimSpace(stdout.String())
	decoded, err := base64.StdEncoding.DecodeString(result)
	if err != nil {
		return nil, fmt.Errorf("vault: failed to decode value: %w", err)
	}
	return decoded, nil
}

func del(service, key string) error {
	cmd := exec.Command("security", "delete-generic-password",
		"-a", key, // account name
		"-s", service, // service name
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := stderr.String()
		if strings.Contains(errStr, "could not be found") ||
			strings.Contains(errStr, "SecKeychainSearchCopyNext") {
			return ErrNotFound
		}
		return fmt.Errorf("vault: failed to delete key: %s", errStr)
	}

	return nil
}
