package storage

import (
	"strconv"
	"strings"
	"unicode/utf16"
)

const SEP byte = 0

func IdxKey(parts ...string) []byte {
	totalSize := len(parts) - 1 // SEPs
	for _, s := range parts {
		totalSize += len(s)
	}

	buffer := make([]byte, totalSize)
	offset := 0

	for i, s := range parts {
		if i > 0 {
			buffer[offset] = SEP
			offset++
		}
		offset += copy(buffer[offset:], s)
	}

	return buffer
}

func Norm(s string) string {
	return strings.ToLower(s)
}

// ID basado en src, label y dst.
func MakeEdgeID(src, label, dst string) string {
	s := src + "|" + label + "|" + dst

	javaChars := utf16.Encode([]rune(s))

	var x uint64 = 1125899906842597

	for i := 0; i < len(javaChars); i++ {
		x = (x * 1315423911) ^ uint64(javaChars[i]) //
	}

	return strconv.FormatUint(x, 10)
}
