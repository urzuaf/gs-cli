package storage

import "encoding/binary"

// ("edge:" + edgeId)
func EncodeEdgeKey(edgeID string) []byte {
	return []byte("edge:" + edgeID)
}

// [u16 idLen][id][u16 labLen][lab][u16 srcLen][src][u16 dstLen][dst][u16 propCount]{ [u16 kLen][k][u32 vLen][v] }
func EncodeEdgeValue(id, label, src, dst string, props map[string]string) []byte {

	//total size
	totalSize := 2 + len(id) + 2 + len(label) + 2 + len(src) + 2 + len(dst) + 2
	for k, v := range props {
		totalSize += 2 + len(k) + 4 + len(v)
	}

	buffer := make([]byte, totalSize)
	offset := 0

	binary.BigEndian.PutUint16(buffer[offset:], uint16(len(id)))
	offset += 2
	offset += copy(buffer[offset:], id)

	binary.BigEndian.PutUint16(buffer[offset:], uint16(len(label)))
	offset += 2
	offset += copy(buffer[offset:], label)

	binary.BigEndian.PutUint16(buffer[offset:], uint16(len(src)))
	offset += 2
	offset += copy(buffer[offset:], src)

	binary.BigEndian.PutUint16(buffer[offset:], uint16(len(dst)))
	offset += 2
	offset += copy(buffer[offset:], dst)

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

func EncodeMegaBlob(edgeB, srcB, dstB []byte) []byte {
	size := 4 + len(edgeB) + 4 + len(srcB) + 4 + len(dstB)
	buffer := make([]byte, size)
	offset := 0

	binary.BigEndian.PutUint32(buffer[offset:], uint32(len(edgeB)))
	offset += 4
	offset += copy(buffer[offset:], edgeB)

	binary.BigEndian.PutUint32(buffer[offset:], uint32(len(srcB)))
	offset += 4
	offset += copy(buffer[offset:], srcB)

	binary.BigEndian.PutUint32(buffer[offset:], uint32(len(dstB)))
	offset += 4
	offset += copy(buffer[offset:], dstB)

	return buffer
}

func EncodeFatBatch(megaBlobs [][]byte) []byte {
	totalSize := 4 // count
	for _, b := range megaBlobs {
		totalSize += 4 + len(b)
	}
	buffer := make([]byte, totalSize)
	offset := 0

	binary.BigEndian.PutUint32(buffer[offset:], uint32(len(megaBlobs)))
	offset += 4

	for _, b := range megaBlobs {
		binary.BigEndian.PutUint32(buffer[offset:], uint32(len(b)))
		offset += 4
		offset += copy(buffer[offset:], b)
	}
	return buffer
}
