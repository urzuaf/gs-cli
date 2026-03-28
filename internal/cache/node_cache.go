package cache

import "sync"

type NodeCache struct {
	mu sync.RWMutex

	labelToInt map[string]uint16
	intToLabel []string

	store map[string]uint16
}

func NewNodeCache() *NodeCache {
	return &NodeCache{
		labelToInt: make(map[string]uint16),
		intToLabel: make([]string, 0, 100),           // Pre-reserve space for 100 labels
		store:      make(map[string]uint16, 1000000), // Reserve space for 1 million nodes
	}
}

func (c *NodeCache) Put(nodeID, label string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// seaarch if label has a name asigned
	labelID, exists := c.labelToInt[label]
	if !exists {
		// if doesnt exist we give it the next name
		labelID = uint16(len(c.intToLabel))
		c.labelToInt[label] = labelID
		c.intToLabel = append(c.intToLabel, label)
	}

	c.store[nodeID] = labelID
}

func (c *NodeCache) Get(nodeID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	labelID, exists := c.store[nodeID]
	if !exists {
		return "", false
	}

	return c.intToLabel[labelID], true
}

func (c *NodeCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = nil
}
