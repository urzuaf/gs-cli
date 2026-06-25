package tests

import (
	"flag"
	"gs-cli/internal/validator"
	"os"
	"testing"
)

var (
	manualNodes = flag.String("nodes", "", "Path to nodes PGDF file for manual test")
	manualEdges = flag.String("edges", "", "Path to edges PGDF file for manual test")
)

func TestValidate(t *testing.T) {
	nodesContent := `@id|@label|creationDate|locationIP|browserUsed|content|length
comment_1|COMMENT|1341766121630|91.191.192.127|Firefox|yes|3
comment_2|COMMENT|1341754323239|91.191.192.127|Firefox|thanks|6
@id|@label|creationDate|title
forum_0|FORUM|1262531441499|Wall of Hossein Forouhar
`
	edgesContent := `@id|@label|@dir|@out|@in|creationDate
edge_1|HasCreator|T|comment_1|forum_0|1341766121630
@id|@label|@dir|@out|@in|creationDate
edge_2|ContainerOf|T|forum_0|comment_2|1311825263934
`

	nodesFile, _ := os.CreateTemp("", "nodes*.pgdf")
	defer os.Remove(nodesFile.Name())
	nodesFile.WriteString(nodesContent)
	nodesFile.Close()

	edgesFile, _ := os.CreateTemp("", "edges*.pgdf")
	defer os.Remove(edgesFile.Name())
	edgesFile.WriteString(edgesContent)
	edgesFile.Close()

	v := validator.NewPGDFValidator()
	err := v.Validate(nodesFile.Name(), edgesFile.Name())
	if err != nil {
		t.Errorf("Validation failed for valid files: %v", err)
	}
}

func TestDuplicateNodeID(t *testing.T) {
	nodesContent := `@id|@label
n1|L
n1|L
`
	nodesFile, _ := os.CreateTemp("", "nodes*.pgdf")
	defer os.Remove(nodesFile.Name())
	nodesFile.WriteString(nodesContent)
	nodesFile.Close()

	edgesFile, _ := os.CreateTemp("", "edges*.pgdf")
	defer os.Remove(edgesFile.Name())
	edgesFile.WriteString("@id|@label|@dir|@out|@in\n")
	edgesFile.Close()

	v := validator.NewPGDFValidator()
	err := v.Validate(nodesFile.Name(), edgesFile.Name())
	if err == nil {
		t.Error("Expected error for duplicate node ID, got nil")
	}
}

func TestReferentialIntegrity(t *testing.T) {
	nodesContent := `@id|@label
n1|L
`
	edgesContent := `@id|@label|@dir|@out|@in
e1|E|T|n1|n2
`
	nodesFile, _ := os.CreateTemp("", "nodes*.pgdf")
	defer os.Remove(nodesFile.Name())
	nodesFile.WriteString(nodesContent)
	nodesFile.Close()

	edgesFile, _ := os.CreateTemp("", "edges*.pgdf")
	defer os.Remove(edgesFile.Name())
	edgesFile.WriteString(edgesContent)
	edgesFile.Close()

	v := validator.NewPGDFValidator()
	err := v.Validate(nodesFile.Name(), edgesFile.Name())
	if err == nil {
		t.Error("Expected error for non-existent target node, got nil")
	}
}

func TestManual(t *testing.T) {
	if *manualNodes == "" || *manualEdges == "" {
		t.Skip("Skipping manual test. Provide -nodes and -edges flags to run.")
	}

	v := validator.NewPGDFValidator()
	err := v.Validate(*manualNodes, *manualEdges)
	if err != nil {
		t.Fatalf("Manual validation failed: %v", err)
	}
}
