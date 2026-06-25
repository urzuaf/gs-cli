package tests

import (
	"bytes"
	"gs-cli/internal/storage"
	"testing"
)

func TestNodeKey(t *testing.T) {
	tests := []struct {
		name     string
		nodeID   string
		expected []byte
	}{
		{
			name:     "ID simple",
			nodeID:   "123",
			expected: []byte("node:123"),
		},
		{
			name:     "ID alfanumérico",
			nodeID:   "aB9_x",
			expected: []byte("node:aB9_x"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storage.EncodeNodeKey(tt.nodeID)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("NodeKey() = %v, se esperaba %v", got, tt.expected)
			}
		})
	}
}

func TestEncodeNodeValue(t *testing.T) {
	// 1. Without properties
	expected1 := []byte{0x00, 0x04, 'U', 's', 'e', 'r', 0x00, 0x00}

	// 2. Multiple properties
	opt1 := []byte{
		0x00, 0x06, 'P', 'e', 'r', 's', 'o', 'n',
		0x00, 0x02,
		0x00, 0x04, 'n', 'a', 'm', 'e',
		0x00, 0x00, 0x00, 0x08, 'F', 'e', 'r', 'n', 'a', 'n', 'd', 'o',
		0x00, 0x03, 'a', 'g', 'e',
		0x00, 0x00, 0x00, 0x02, '2', '4',
	}
	opt2 := []byte{
		0x00, 0x06, 'P', 'e', 'r', 's', 'o', 'n',
		0x00, 0x02,
		0x00, 0x03, 'a', 'g', 'e',
		0x00, 0x00, 0x00, 0x02, '2', '4',
		0x00, 0x04, 'n', 'a', 'm', 'e',
		0x00, 0x00, 0x00, 0x08, 'F', 'e', 'r', 'n', 'a', 'n', 'd', 'o',
	}

	// 3. Empty value property
	expected3 := []byte{
		0x00, 0x03, 'T', 'a', 'g',
		0x00, 0x01,
		0x00, 0x04, 'd', 'e', 's', 'c',
		0x00, 0x00, 0x00, 0x00,
	}

	t.Run("Nodo sin propiedades", func(t *testing.T) {
		got := storage.EncodeNodeValue("User", map[string]string{})
		if !bytes.Equal(got, expected1) {
			t.Errorf("Mismatch. Got %v, expected %v", got, expected1)
		}
	})

	t.Run("Nodo con múltiples propiedades", func(t *testing.T) {
		got := storage.EncodeNodeValue("Person", map[string]string{"name": "Fernando", "age": "24"})
		if !bytes.Equal(got, opt1) && !bytes.Equal(got, opt2) {
			t.Errorf("Mismatch. Got %v, expected either %v or %v", got, opt1, opt2)
		}
	})

	t.Run("Caso borde: Propiedad con valor vacío", func(t *testing.T) {
		got := storage.EncodeNodeValue("Tag", map[string]string{"desc": ""})
		if !bytes.Equal(got, expected3) {
			t.Errorf("Mismatch. Got %v, expected %v", got, expected3)
		}
	})
}
