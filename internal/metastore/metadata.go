package metastore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type EdgeConnectionStats struct {
	SrcLabel string `json:"srcLabel"`
	DstLabel string `json:"dstLabel"`
	Count    int64  `json:"count"`
}

type MetaData struct {
	NodeCount        int64                            `json:"nodeCount"`
	EdgeCount        int64                            `json:"edgeCount"`
	NodeCountByLabel map[string]int64                 `json:"nodeCountByLabel"`
	EdgeCountByLabel map[string]int64                 `json:"edgeCountByLabel"`
	NodeSchema       map[string][]string              `json:"nodeSchema"`
	EdgeSchema       map[string][]string              `json:"edgeSchema"`
	EdgeConnections  map[string][]EdgeConnectionStats `json:"edgeConnections"`
}

type MetaStore struct {
	mu sync.RWMutex

	nodeCount        int64
	edgeCount        int64
	nodeCountByLabel map[string]int64
	edgeCountByLabel map[string]int64

	nodeSchema map[string]map[string]struct{}
	edgeSchema map[string]map[string]struct{}

	edgeConnections map[string]map[string]int64
}

func NewMetaStore() *MetaStore {
	return &MetaStore{
		nodeCountByLabel: make(map[string]int64),
		edgeCountByLabel: make(map[string]int64),
		nodeSchema:       make(map[string]map[string]struct{}),
		edgeSchema:       make(map[string]map[string]struct{}),
		edgeConnections:  make(map[string]map[string]int64),
	}
}

// add nodes and register properties
func (m *MetaStore) IncNode(label string, props []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodeCount++
	m.nodeCountByLabel[label]++

	if _, exists := m.nodeSchema[label]; !exists {
		m.nodeSchema[label] = make(map[string]struct{})
	}
	for _, p := range props {
		m.nodeSchema[label][p] = struct{}{}
	}
}

// add endges and register their props and connections
func (m *MetaStore) IncEdge(label, srcLabel, dstLabel string, props []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.edgeCount++
	m.edgeCountByLabel[label]++

	if _, exists := m.edgeSchema[label]; !exists {
		m.edgeSchema[label] = make(map[string]struct{})
	}
	for _, p := range props {
		m.edgeSchema[label][p] = struct{}{}
	}

	if _, exists := m.edgeConnections[label]; !exists {
		m.edgeConnections[label] = make(map[string]int64)
	}
	connKey := srcLabel + "|" + dstLabel
	m.edgeConnections[label][connKey]++
}

// Save
func (m *MetaStore) Save(dbPath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := MetaData{
		NodeCount:        m.nodeCount,
		EdgeCount:        m.edgeCount,
		NodeCountByLabel: m.nodeCountByLabel,
		EdgeCountByLabel: m.edgeCountByLabel,
		NodeSchema:       make(map[string][]string),
		EdgeSchema:       make(map[string][]string),
		EdgeConnections:  make(map[string][]EdgeConnectionStats),
	}

	for label, propsMap := range m.nodeSchema {
		props := make([]string, 0, len(propsMap))
		for p := range propsMap {
			props = append(props, p)
		}
		data.NodeSchema[label] = props
	}

	for label, propsMap := range m.edgeSchema {
		props := make([]string, 0, len(propsMap))
		for p := range propsMap {
			props = append(props, p)
		}
		data.EdgeSchema[label] = props
	}

	for edgeLabel, conns := range m.edgeConnections {
		stats := make([]EdgeConnectionStats, 0, len(conns))
		for connKey, count := range conns {
			for i := 0; i < len(connKey); i++ {
				if connKey[i] == '|' {
					stats = append(stats, EdgeConnectionStats{
						SrcLabel: connKey[:i],
						DstLabel: connKey[i+1:],
						Count:    count,
					})
					break
				}
			}
		}
		data.EdgeConnections[edgeLabel] = stats
	}

	fileBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	metaFile := filepath.Join(dbPath, "metadata.json")
	return os.WriteFile(metaFile, fileBytes, 0644)
}
