package tests

import (
	"gs-cli/internal/parser"
	"reflect"
	"testing"
)

func TestParseHeader(t *testing.T) {
	line := "@id | @label| name |age "
	expected := []string{"@id", "@label", "name", "age"}

	got := parser.ParseHeader(line)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ParseHeader() = %v, se esperaba %v", got, expected)
	}
}

func TestFastSplit(t *testing.T) {
	line := " 123 | Person | Fernando | 24 "
	buffer := make([]string, 0, 10)

	expected := []string{" 123 ", " Person ", " Fernando ", " 24 "}

	got := parser.FastSplit(line, buffer)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("FastSplit() = %v, se esperaba %v", got, expected)
	}
}

func TestParseLine_Node(t *testing.T) {
	header := []string{"@id", "@label", "name", "age"}
	line := "node1 | User | Fernando | 24"
	buffer := make([]string, 0, 10)

	expected := parser.Record{
		ID:    "node1",
		Label: "User",
		Props: map[string]string{
			"name": "Fernando",
			"age":  "24",
		},
	}

	got := parser.ParseLine(line, header, buffer)

	if got.ID != expected.ID || got.Label != expected.Label || !reflect.DeepEqual(got.Props, expected.Props) {
		t.Errorf("ParseLine (Node) falló.\nObtuviste: %+v\nEsperabas: %+v", got, expected)
	}
}

func TestParseLine_Edge(t *testing.T) {
	header := []string{"@out", "@label", "@in", "@dir", "weight"}
	line := "user1 | KNOWS | user2 | T | 0.8"
	buffer := make([]string, 0, 10)

	expected := parser.Record{
		Src:   "user1",
		Label: "KNOWS",
		Dst:   "user2",
		Dir:   "T",
		Props: map[string]string{
			"weight": "0.8",
		},
	}

	got := parser.ParseLine(line, header, buffer)

	if got.Src != expected.Src || got.Label != expected.Label || got.Dst != expected.Dst || got.Dir != expected.Dir || !reflect.DeepEqual(got.Props, expected.Props) {
		t.Errorf("ParseLine (Edge) falló.\nObtuviste: %+v\nEsperabas: %+v", got, expected)
	}
}
