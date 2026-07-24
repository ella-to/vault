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

const (
	dbName = "vault-secrets"
	// dbVersion 2: version 1 databases may exist without the object store
	// (created by earlier releases); the upgrade callback recreates it.
	dbVersion = 2
	storeName = "secrets"
)

func set(service, key string, value []byte) error {
	encoded := base64.StdEncoding.EncodeToString(value)
	storeKey := service + "/" + key

	return withStore("readwrite", func(store js.Value, cb *callbacks, done chan<- error) {
		request := store.Call("put", encoded, storeKey)

		request.Set("onsuccess", cb.of(func(this js.Value, args []js.Value) any {
			done <- nil
			return nil
		}))

		request.Set("onerror", cb.of(func(this js.Value, args []js.Value) any {
			done <- errors.New("vault: failed to set key in IndexedDB")
			return nil
		}))
	})
}

func get(service, key string) ([]byte, error) {
	storeKey := service + "/" + key
	var result []byte

	err := withStore("readonly", func(store js.Value, cb *callbacks, done chan<- error) {
		request := store.Call("get", storeKey)

		request.Set("onsuccess", cb.of(func(this js.Value, args []js.Value) any {
			res := request.Get("result")
			if res.IsUndefined() || res.IsNull() {
				done <- ErrNotFound
				return nil
			}

			decoded, err := base64.StdEncoding.DecodeString(res.String())
			if err != nil {
				done <- err
				return nil
			}
			result = decoded
			done <- nil
			return nil
		}))

		request.Set("onerror", cb.of(func(this js.Value, args []js.Value) any {
			done <- errors.New("vault: failed to get key from IndexedDB")
			return nil
		}))
	})

	return result, err
}

func del(service, key string) error {
	storeKey := service + "/" + key

	return withStore("readwrite", func(store js.Value, cb *callbacks, done chan<- error) {
		// First check if key exists
		getRequest := store.Call("get", storeKey)

		getRequest.Set("onsuccess", cb.of(func(this js.Value, args []js.Value) any {
			res := getRequest.Get("result")
			if res.IsUndefined() || res.IsNull() {
				done <- ErrNotFound
				return nil
			}

			// Key exists, delete it. The transaction is still active while
			// its event handlers run, so issuing the request here is valid.
			deleteRequest := store.Call("delete", storeKey)

			deleteRequest.Set("onsuccess", cb.of(func(this js.Value, args []js.Value) any {
				done <- nil
				return nil
			}))

			deleteRequest.Set("onerror", cb.of(func(this js.Value, args []js.Value) any {
				done <- errors.New("vault: failed to delete key from IndexedDB")
				return nil
			}))

			return nil
		}))

		getRequest.Set("onerror", cb.of(func(this js.Value, args []js.Value) any {
			done <- errors.New("vault: failed to check key in IndexedDB")
			return nil
		}))
	})
}

// callbacks tracks js.FuncOf wrappers so they can be released once an
// operation completes, instead of leaking one per call.
type callbacks struct {
	list []js.Func
}

func (c *callbacks) of(fn func(this js.Value, args []js.Value) any) js.Func {
	f := js.FuncOf(fn)
	c.list = append(c.list, f)
	return f
}

func (c *callbacks) release() {
	for _, f := range c.list {
		f.Release()
	}
}

// withStore opens the database and calls fn with an object store.
//
// fn runs inside the open request's success event handler. Blocking there
// would pause the JavaScript event loop and deadlock (see js.FuncOf docs),
// so fn must issue its IndexedDB requests synchronously and return without
// waiting, delivering the outcome on done. Only this function — running on
// the caller's goroutine, outside any event handler — blocks on done.
func withStore(mode string, fn func(store js.Value, cb *callbacks, done chan<- error)) error {
	indexedDB := js.Global().Get("indexedDB")
	if indexedDB.IsUndefined() || indexedDB.IsNull() {
		return errors.New("vault: IndexedDB is not available in this environment")
	}

	done := make(chan error, 1)
	cb := &callbacks{}
	defer cb.release()

	request := indexedDB.Call("open", dbName, dbVersion)

	request.Set("onupgradeneeded", cb.of(func(this js.Value, args []js.Value) any {
		db := request.Get("result")
		if !db.Get("objectStoreNames").Call("contains", storeName).Bool() {
			db.Call("createObjectStore", storeName)
		}
		return nil
	}))

	request.Set("onsuccess", cb.of(func(this js.Value, args []js.Value) any {
		db := request.Get("result")
		tx := db.Call("transaction", storeName, mode)
		store := tx.Call("objectStore", storeName)

		fn(store, cb, done)

		// close waits for active transactions to finish before closing.
		db.Call("close")
		return nil
	}))

	request.Set("onerror", cb.of(func(this js.Value, args []js.Value) any {
		done <- errors.New("vault: failed to open IndexedDB")
		return nil
	}))

	return <-done
}
