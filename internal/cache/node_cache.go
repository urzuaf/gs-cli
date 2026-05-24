package cache

import "sync"

type NodeCache struct {
	mu sync.RWMutex

	labelToInt map[string]uint16
	intToLabel []string

	store map[string][]byte
}

func NewNodeCache() *NodeCache {
	return &NodeCache{
		labelToInt: make(map[string]uint16),
		intToLabel: make([]string, 0, 100),           // Pre-reserve space for 100 labels
		store:      make(map[string][]byte, 1000000), // Reserve space for 1 million nodes
	}
}

func (c *NodeCache) Put(nodeID string, blob []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[nodeID] = blob
}

func (c *NodeCache) Get(nodeID string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	blob, exists := c.store[nodeID]
	return blob, exists
}

func (c *NodeCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string][]byte)
}
