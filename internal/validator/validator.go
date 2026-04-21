package validator

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type PGDFValidator struct {
	nodeIDs map[string]bool
	edgeIDs map[string]bool
}

func NewPGDFValidator() *PGDFValidator {
	return &PGDFValidator{
		nodeIDs: make(map[string]bool),
		edgeIDs: make(map[string]bool),
	}
}

func (v *PGDFValidator) Validate(nodesPath, edgesPath string) error {
	fmt.Printf("Validating nodes file: %s\n", nodesPath)
	if err := v.validateNodes(nodesPath); err != nil {
		return err
	}

	fmt.Printf("Validating edges file: %s\n", edgesPath)
	if err := v.validateEdges(edgesPath); err != nil {
		return err
	}

	fmt.Println("Validation successful!")
	return nil
}

func (v *PGDFValidator) validateNodes(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening nodes file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	var currentHeader []string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if isHeader(line) {
			currentHeader = strings.Split(line, "|")
			for i := range currentHeader {
				currentHeader[i] = strings.TrimSpace(currentHeader[i])
			}
			// Validate header has mandatory fields
			if !hasField(currentHeader, "@id") || !hasField(currentHeader, "@label") {
				return fmt.Errorf("line %d: Node header must contain @id and @label", lineNum)
			}
			continue
		}

		if currentHeader == nil {
			return fmt.Errorf("line %d: Data found before header", lineNum)
		}

		parts := strings.Split(line, "|")
		if len(parts) != len(currentHeader) {
			return fmt.Errorf("line %d: Row has %d columns, but header has %d", lineNum, len(parts), len(currentHeader))
		}

		// Extract ID
		id := ""
		for i, field := range currentHeader {
			if field == "@id" {
				id = strings.TrimSpace(parts[i])
				break
			}
		}

		if id == "" {
			return fmt.Errorf("line %d: Missing @id value", lineNum)
		}

		if v.nodeIDs[id] {
			return fmt.Errorf("line %d: Duplicate node ID: %s", lineNum, id)
		}
		v.nodeIDs[id] = true
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading nodes file: %v", err)
	}

	return nil
}

func (v *PGDFValidator) validateEdges(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening edges file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	var currentHeader []string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if isHeader(line) {
			currentHeader = strings.Split(line, "|")
			for i := range currentHeader {
				currentHeader[i] = strings.TrimSpace(currentHeader[i])
			}
			// Validate header has mandatory fields
			mandatory := []string{"@id", "@label", "@out", "@in", "@dir"}
			for _, m := range mandatory {
				if !hasField(currentHeader, m) {
					return fmt.Errorf("line %d: Edge header must contain %s", lineNum, m)
				}
			}
			continue
		}

		if currentHeader == nil {
			return fmt.Errorf("line %d: Data found before header", lineNum)
		}

		parts := strings.Split(line, "|")
		if len(parts) != len(currentHeader) {
			return fmt.Errorf("line %d: Row has %d columns, but header has %d", lineNum, len(parts), len(currentHeader))
		}

		var id, out, in string
		for i, field := range currentHeader {
			val := strings.TrimSpace(parts[i])
			switch field {
			case "@id":
				id = val
			case "@out":
				out = val
			case "@in":
				in = val
			}
		}

		if id == "" {
			return fmt.Errorf("line %d: Missing @id value", lineNum)
		}
		if v.edgeIDs[id] {
			return fmt.Errorf("line %d: Duplicate edge ID: %s", lineNum, id)
		}
		v.edgeIDs[id] = true

		if !v.nodeIDs[out] {
			return fmt.Errorf("line %d: Edge %s refers to non-existent source node: %s", lineNum, id, out)
		}
		if !v.nodeIDs[in] {
			return fmt.Errorf("line %d: Edge %s refers to non-existent target node: %s", lineNum, id, in)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading edges file: %v", err)
	}

	return nil
}

func isHeader(line string) bool {
	// A header is a line where at least one field starts with @
	parts := strings.Split(line, "|")
	for _, p := range parts {
		if strings.HasPrefix(strings.TrimSpace(p), "@") {
			return true
		}
	}
	return false
}

func hasField(header []string, field string) bool {
	for _, f := range header {
		if f == field {
			return true
		}
	}
	return false
}
