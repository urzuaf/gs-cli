package storage

import "encoding/binary"

func EncodeNodeKey(nodeID string) []byte {
	return []byte("node:" + nodeID)
}

// [u16 labLen][lab][u16 propCount]{ [u16 kLen][k][u32 vLen][v] }
func EncodeNodeValue(label string, props map[string]string) []byte {

	totalSize := 4 + len(label)
	for k, v := range props {
		totalSize += 6 + len(k) + len(v)
	}

	buffer := make([]byte, totalSize)
	offset := 0

	binary.BigEndian.PutUint16(buffer[offset:], uint16(len(label)))
	offset += 2
	offset += copy(buffer[offset:], label)

	binary.BigEndian.PutUint16(buffer[offset:], uint16(len(props)))
	offset += 2

	for k, v := range props {
		binary.BigEndian.PutUint16(buffer[offset:], uint16(len(k)))
		offset += 2
		offset += copy(buffer[offset:], k)

		binary.BigEndian.PutUint32(buffer[offset:], uint32(len(v)))
		offset += 4
		offset += copy(buffer[offset:], v)
	}

	return buffer

}
