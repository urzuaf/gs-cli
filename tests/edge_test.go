package tests

import (
	"bytes"
	"gs-cli/internal/storage"
	"testing"
)

func TestEncodeEdgeKey(t *testing.T) {
	tests := []struct {
		name     string
		edgeID   string
		expected []byte
	}{
		{
			name:     "ID de arista numérico",
			edgeID:   "987654321",
			expected: []byte("edge:987654321"),
		},
		{
			name:     "ID de arista compuesto",
			edgeID:   "user1|KNOWS|user2",
			expected: []byte("edge:user1|KNOWS|user2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storage.EncodeEdgeKey(tt.edgeID)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("EncodeEdgeKey() = %v, se esperaba %v", got, tt.expected)
			}
		})
	}
}

func TestEncodeEdgeValue(t *testing.T) {
	// e1 without properties
	e1Expected := []byte{
		0x00, 0x02, 'e', '1',
		0x00, 0x05, 'K', 'N', 'O', 'W', 'S',
		0x00, 0x05, 'u', 's', 'e', 'r', '1',
		0x00, 0x05, 'u', 's', 'e', 'r', '2',
		0x00, 0x00,
	}

	// e2 with single property
	e2Expected := []byte{
		0x00, 0x02, 'e', '2',
		0x00, 0x05, 'L', 'I', 'K', 'E', 'S',
		0x00, 0x02, '1', '0',
		0x00, 0x02, '2', '0',
		0x00, 0x01,
		0x00, 0x06, 'w', 'e', 'i', 'g', 'h', 't',
		0x00, 0x00, 0x00, 0x03, '0', '.', '8',
	}

	// e3 with multiple properties
	e3Opt1 := []byte{
		0x00, 0x02, 'e', '3',
		0x00, 0x05, 'V', 'I', 'S', 'T', 'O',
		0x00, 0x01, 'a',
		0x00, 0x01, 'b',
		0x00, 0x02,
		0x00, 0x04, 'a', 0xC3, 0xB1, 'o',
		0x00, 0x00, 0x00, 0x04, '2', '0', '2', '6',
		0x00, 0x05, 'f', 'e', 'c', 'h', 'a',
		0x00, 0x00, 0x00, 0x00,
	}
	e3Opt2 := []byte{
		0x00, 0x02, 'e', '3',
		0x00, 0x05, 'V', 'I', 'S', 'T', 'O',
		0x00, 0x01, 'a',
		0x00, 0x01, 'b',
		0x00, 0x02,
		0x00, 0x05, 'f', 'e', 'c', 'h', 'a',
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x04, 'a', 0xC3, 0xB1, 'o',
		0x00, 0x00, 0x00, 0x04, '2', '0', '2', '6',
	}

	t.Run("Arista sin propiedades", func(t *testing.T) {
		got := storage.EncodeEdgeValue("e1", "KNOWS", "user1", "user2", map[string]string{})
		if !bytes.Equal(got, e1Expected) {
			t.Errorf("Mismatch. Got %v, expected %v", got, e1Expected)
		}
	})

	t.Run("Arista con propiedades", func(t *testing.T) {
		got := storage.EncodeEdgeValue("e2", "LIKES", "10", "20", map[string]string{"weight": "0.8"})
		if !bytes.Equal(got, e2Expected) {
			t.Errorf("Mismatch. Got %v, expected %v", got, e2Expected)
		}
	})

	t.Run("Caso borde: Caracteres UTF-8 y valores vacíos", func(t *testing.T) {
		got := storage.EncodeEdgeValue("e3", "VISTO", "a", "b", map[string]string{"año": "2026", "fecha": ""})
		if !bytes.Equal(got, e3Opt1) && !bytes.Equal(got, e3Opt2) {
			t.Errorf("Mismatch. Got %v, expected either %v or %v", got, e3Opt1, e3Opt2)
		}
	})
}
