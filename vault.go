// Package vault provides a simple interface to securely store, retrieve,
// and delete secrets using platform-native secure storage.
package vault

import "errors"

var (
	// ErrNotFound is returned when a key is not found in the vault.
	ErrNotFound = errors.New("vault: key not found")

	// ErrInvalidKey is returned when a key is empty or invalid.
	ErrInvalidKey = errors.New("vault: invalid key")

	// ErrInvalidValue is returned when a value is empty or invalid.
	ErrInvalidValue = errors.New("vault: invalid value")
)

// Set stores a value securely in the platform's native secure storage.
// The service parameter is used to namespace the keys.
func Set(service, key string, value []byte) error {
	if service == "" || key == "" {
		return ErrInvalidKey
	}
	if len(value) == 0 {
		return ErrInvalidValue
	}
	return set(service, key, value)
}

// Get retrieves a value from the platform's native secure storage.
// Returns ErrNotFound if the key does not exist.
func Get(service, key string) ([]byte, error) {
	if service == "" || key == "" {
		return nil, ErrInvalidKey
	}
	return get(service, key)
}

// Del removes a value from the platform's native secure storage.
// Returns ErrNotFound if the key does not exist.
func Del(service, key string) error {
	if service == "" || key == "" {
		return ErrInvalidKey
	}
	return del(service, key)
}
