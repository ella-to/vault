package vault

import (
	"testing"
)

const testService = "vault-test-service"

func TestSetGetDel(t *testing.T) {
	key := "test-key"
	value := []byte("test-secret-value")

	// Clean up any existing key
	_ = Del(testService, key)

	// Test Set
	if err := Set(testService, key, value); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get
	got, err := Get(testService, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}

	// Test Del
	if err = Del(testService, key); err != nil {
		t.Fatalf("Del failed: %v", err)
	}

	// Verify deletion
	_, err = Get(testService, key)
	if err != ErrNotFound {
		t.Errorf("Get after Del returned error %v, want ErrNotFound", err)
	}
}

func TestSetOverwrite(t *testing.T) {
	key := "test-overwrite-key"
	value1 := []byte("first-value")
	value2 := []byte("second-value")

	// Clean up
	defer Del(testService, key)
	_ = Del(testService, key)

	// Set first value
	if err := Set(testService, key, value1); err != nil {
		t.Fatalf("Set first value failed: %v", err)
	}

	// Overwrite with second value
	if err := Set(testService, key, value2); err != nil {
		t.Fatalf("Set second value failed: %v", err)
	}

	// Verify overwrite
	got, err := Get(testService, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(got) != string(value2) {
		t.Errorf("Get returned %q, want %q", got, value2)
	}
}

func TestGetNotFound(t *testing.T) {
	_, err := Get(testService, "nonexistent-key-12345")
	if err != ErrNotFound {
		t.Errorf("Get nonexistent key returned %v, want ErrNotFound", err)
	}
}

func TestDelNotFound(t *testing.T) {
	err := Del(testService, "nonexistent-key-12345")
	if err != ErrNotFound {
		t.Errorf("Del nonexistent key returned %v, want ErrNotFound", err)
	}
}

func TestInvalidInputs(t *testing.T) {
	tests := []struct {
		name    string
		service string
		key     string
		value   []byte
		wantErr error
	}{
		{"empty service", "", "key", []byte("value"), ErrInvalidKey},
		{"empty key", "service", "", []byte("value"), ErrInvalidKey},
		{"empty value", "service", "key", nil, ErrInvalidValue},
		{"empty value slice", "service", "key", []byte{}, ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Set(tt.service, tt.key, tt.value)
			if err != tt.wantErr {
				t.Errorf("Set(%q, %q, %v) = %v, want %v", tt.service, tt.key, tt.value, err, tt.wantErr)
			}
		})
	}

	// Test Get with invalid inputs
	if _, err := Get("", "key"); err != ErrInvalidKey {
		t.Errorf("Get with empty service = %v, want ErrInvalidKey", err)
	}
	if _, err := Get("service", ""); err != ErrInvalidKey {
		t.Errorf("Get with empty key = %v, want ErrInvalidKey", err)
	}

	// Test Del with invalid inputs
	if err := Del("", "key"); err != ErrInvalidKey {
		t.Errorf("Del with empty service = %v, want ErrInvalidKey", err)
	}
	if err := Del("service", ""); err != ErrInvalidKey {
		t.Errorf("Del with empty key = %v, want ErrInvalidKey", err)
	}
}

func TestBinaryData(t *testing.T) {
	key := "test-binary-key"
	// Binary data including null bytes and non-UTF8 sequences
	value := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x80, 0x7F}

	// Clean up
	defer Del(testService, key)
	_ = Del(testService, key)

	if err := Set(testService, key, value); err != nil {
		t.Fatalf("Set binary data failed: %v", err)
	}

	got, err := Get(testService, key)
	if err != nil {
		t.Fatalf("Get binary data failed: %v", err)
	}

	if len(got) != len(value) {
		t.Errorf("Get returned %d bytes, want %d", len(got), len(value))
	}
	for i := range value {
		if got[i] != value[i] {
			t.Errorf("byte %d: got %#x, want %#x", i, got[i], value[i])
		}
	}
}

func TestSpecialCharacters(t *testing.T) {
	key := "test-special-key"
	value := []byte("hello 世界 🌍 \t\n\r special!@#$%^&*()")

	// Clean up
	defer Del(testService, key)
	_ = Del(testService, key)

	if err := Set(testService, key, value); err != nil {
		t.Fatalf("Set special characters failed: %v", err)
	}

	got, err := Get(testService, key)
	if err != nil {
		t.Fatalf("Get special characters failed: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("Get returned %q, want %q", got, value)
	}
}
