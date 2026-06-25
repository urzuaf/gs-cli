package tests

import (
	"bytes"
	"gs-cli/internal/storage"
	"testing"
)

func TestIdxKey(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected []byte
	}{
		{
			name:     "Un solo argumento",
			parts:    []string{"label"},
			expected: []byte{'l', 'a', 'b', 'e', 'l'},
		},
		{
			name:  "Múltiples argumentos",
			parts: []string{"prop", "age", "24", "node:1"},
			expected: []byte{
				'p', 'r', 'o', 'p', 0x00,
				'a', 'g', 'e', 0x00,
				'2', '4', 0x00,
				'n', 'o', 'd', 'e', ':', '1',
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storage.IdxKey(tt.parts...)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("IdxKey() = %v, se esperaba %v", got, tt.expected)
			}
		})
	}
}
