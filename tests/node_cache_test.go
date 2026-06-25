package tests

import (
	"bytes"
	"gs-cli/internal/cache"
	"testing"
)

func TestNodeCache(t *testing.T) {
	c := cache.NewNodeCache()

	// 1. Insertion
	c.Put("node:1", []byte("UserBlob"))
	c.Put("node:2", []byte("PostBlob"))

	// 2. Retrieval
	blob, ok := c.Get("node:1")
	if !ok || !bytes.Equal(blob, []byte("UserBlob")) {
		t.Errorf("Get(node:1) = %s, %v; expected 'UserBlob', true", string(blob), ok)
	}

	blob, ok = c.Get("node:2")
	if !ok || !bytes.Equal(blob, []byte("PostBlob")) {
		t.Errorf("Get(node:2) = %s, %v; expected 'PostBlob', true", string(blob), ok)
	}

	// 3. Clear
	c.Clear()
	_, ok = c.Get("node:1")
	if ok {
		t.Errorf("Get(node:1) returned true after Clear()")
	}
}
