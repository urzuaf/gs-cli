package tests

import (
	"bytes"
	"gs-cli/internal/rocks"
	"os"
	"testing"
)

func TestRocksBasicReadWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rocks_test*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Open DB (writable)
	store, err := rocks.Open(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to open RocksDB: %v", err)
	}

	key := []byte("node:n1")
	value := []byte("id:n1|label:Person|name:Alice")

	// Write to Nodes Column Family
	err = store.DB.PutCF(store.WO, store.CFNodes, key, value)
	if err != nil {
		store.Close()
		t.Fatalf("Failed to put key-value: %v", err)
	}
	store.Close()

	// Reopen DB (read-only)
	storeReadOnly, err := rocks.Open(tempDir, true)
	if err != nil {
		t.Fatalf("Failed to open RocksDB read-only: %v", err)
	}
	defer storeReadOnly.Close()

	// Read back
	retrievedValue, err := storeReadOnly.DB.GetCF(storeReadOnly.RO, storeReadOnly.CFNodes, key)
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}
	defer retrievedValue.Free()

	if !bytes.Equal(retrievedValue.Data(), value) {
		t.Errorf("Value mismatch. Expected %q, got %q", string(value), string(retrievedValue.Data()))
	}

	// Verify Column Family handles are loaded and non-nil
	if storeReadOnly.CFNodes == nil || storeReadOnly.CFEdges == nil || storeReadOnly.CFIdxNodeProp == nil {
		t.Error("Expected all column family handles to be loaded and non-nil")
	}
}
