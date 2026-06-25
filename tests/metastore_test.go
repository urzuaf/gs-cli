package tests

import (
	"gs-cli/internal/metastore"
	"os"
	"testing"
)

func TestMetastoreIncNodeAndEdge(t *testing.T) {
	m := metastore.NewMetaStore()

	// Register a couple of nodes
	m.IncNode("Person", []string{"name", "age"})
	m.IncNode("Person", []string{"name", "gender"})
	m.IncNode("Message", []string{"text"})

	// Register an edge
	m.IncEdge("knows", "Person", "Person", []string{"since"})

	// Create temp dir for save/load
	tempDir, err := os.MkdirTemp("", "metastore_test*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save metadata
	if err := m.Save(tempDir); err != nil {
		t.Fatalf("Failed to save metastore: %v", err)
	}

	// Load metadata
	data, err := metastore.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load metastore: %v", err)
	}

	// Assertions
	if data.NodeCount != 3 {
		t.Errorf("Expected 3 nodes, got %d", data.NodeCount)
	}
	if data.EdgeCount != 1 {
		t.Errorf("Expected 1 edge, got %d", data.EdgeCount)
	}
	if data.NodeCountByLabel["Person"] != 2 {
		t.Errorf("Expected 2 Person nodes, got %d", data.NodeCountByLabel["Person"])
	}
	if data.EdgeCountByLabel["knows"] != 1 {
		t.Errorf("Expected 1 knows edge, got %d", data.EdgeCountByLabel["knows"])
	}

	// Schemas
	personProps := data.NodeSchema["Person"]
	if len(personProps) != 3 { // name, age, gender
		t.Errorf("Expected 3 properties for Person, got %d: %v", len(personProps), personProps)
	}

	// Connections
	conns := data.EdgeConnections["knows"]
	if len(conns) != 1 {
		t.Fatalf("Expected 1 connection for knows, got %d", len(conns))
	}
	if conns[0].SrcLabel != "Person" || conns[0].DstLabel != "Person" || conns[0].Count != 1 {
		t.Errorf("Unexpected connection stats: %+v", conns[0])
	}
}
