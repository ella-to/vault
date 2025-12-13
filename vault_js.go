//go:build js && wasm

package vault

import (
	"encoding/base64"
	"errors"
	"syscall/js"
)

// WASM/Browser implementation using IndexedDB for storage.
// Values are base64 encoded for safe storage.
//
// Note: Browser storage is NOT as secure as native keychains:
// - Data is accessible to JavaScript running on the same origin
// - No hardware-backed encryption
// - Cleared when user clears browser data
//
// For better security, consider:
// - Using Web Crypto API to encrypt values before storage
// - Server-side secret management for sensitive credentials

var (
	indexedDB js.Value
	dbName    = "vault-secrets"
	storeName = "secrets"
)

func init() {
	indexedDB = js.Global().Get("indexedDB")
}

func set(service, key string, value []byte) error {
	encoded := base64.StdEncoding.EncodeToString(value)
	storeKey := service + "/" + key

	return withStore("readwrite", func(store js.Value) error {
		done := make(chan error, 1)

		request := store.Call("put", map[string]any{
			"key":   storeKey,
			"value": encoded,
		}, storeKey)

		request.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
			done <- nil
			return nil
		}))

		request.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			done <- errors.New("vault: failed to set key in IndexedDB")
			return nil
		}))

		return <-done
	})
}

func get(service, key string) ([]byte, error) {
	storeKey := service + "/" + key
	var result []byte

	err := withStore("readonly", func(store js.Value) error {
		done := make(chan error, 1)

		request := store.Call("get", storeKey)

		request.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
			res := request.Get("result")
			if res.IsUndefined() || res.IsNull() {
				done <- ErrNotFound
				return nil
			}

			encoded := res.Get("value").String()
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				done <- err
				return nil
			}
			result = decoded
			done <- nil
			return nil
		}))

		request.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			done <- errors.New("vault: failed to get key from IndexedDB")
			return nil
		}))

		return <-done
	})

	return result, err
}

func del(service, key string) error {
	storeKey := service + "/" + key

	return withStore("readwrite", func(store js.Value) error {
		done := make(chan error, 1)

		// First check if key exists
		getRequest := store.Call("get", storeKey)

		getRequest.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
			res := getRequest.Get("result")
			if res.IsUndefined() || res.IsNull() {
				done <- ErrNotFound
				return nil
			}

			// Key exists, delete it
			deleteRequest := store.Call("delete", storeKey)

			deleteRequest.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
				done <- nil
				return nil
			}))

			deleteRequest.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
				done <- errors.New("vault: failed to delete key from IndexedDB")
				return nil
			}))

			return nil
		}))

		getRequest.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			done <- errors.New("vault: failed to check key in IndexedDB")
			return nil
		}))

		return <-done
	})
}

// withStore opens the database and executes fn with an object store
func withStore(mode string, fn func(store js.Value) error) error {
	done := make(chan error, 1)

	request := indexedDB.Call("open", dbName, 1)

	request.Set("onupgradeneeded", js.FuncOf(func(this js.Value, args []js.Value) any {
		db := request.Get("result")
		if !db.Call("objectStoreNames").Call("contains", storeName).Bool() {
			db.Call("createObjectStore", storeName, map[string]any{
				"keyPath": "key",
			})
		}
		return nil
	}))

	request.Set("onsuccess", js.FuncOf(func(this js.Value, args []js.Value) any {
		db := request.Get("result")
		tx := db.Call("transaction", storeName, mode)
		store := tx.Call("objectStore", storeName)

		err := fn(store)

		tx.Set("oncomplete", js.FuncOf(func(this js.Value, args []js.Value) any {
			db.Call("close")
			return nil
		}))

		done <- err
		return nil
	}))

	request.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		done <- errors.New("vault: failed to open IndexedDB")
		return nil
	}))

	return <-done
}
