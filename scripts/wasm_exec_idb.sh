#!/bin/sh
# Runs a Go js/wasm test binary under Node.js with an IndexedDB polyfill.
#
# Usage: GOOS=js GOARCH=wasm go test -exec "$PWD/scripts/wasm_exec_idb.sh" ./...
# Requires: node and `npm install --no-save fake-indexeddb` in the repo root.
set -e

GOROOT="$(go env GOROOT)"
WASM_EXEC="$GOROOT/lib/wasm/wasm_exec_node.js"
if [ ! -f "$WASM_EXEC" ]; then
	# Go < 1.24 shipped the helper under misc/wasm
	WASM_EXEC="$GOROOT/misc/wasm/wasm_exec_node.js"
fi

exec node --require fake-indexeddb/auto "$WASM_EXEC" "$@"
