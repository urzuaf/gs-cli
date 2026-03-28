package parser

import (
	"strings"
)

type Record struct {
	ID    string
	Label string
	Src   string
	Dst   string
	Dir   string
	Props map[string]string
}

func ParseHeader(line string) []string {
	parts := strings.Split(line, "|")
	for i := 0; i < len(parts); i++ {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func FastSplit(text string, buffer []string) []string {
	buffer = buffer[:0]
	start := 0

	for i := 0; i < len(text); i++ {
		if text[i] == '|' {
			buffer = append(buffer, text[start:i])
			start = i + 1
		}
	}
	buffer = append(buffer, text[start:])
	return buffer
}

func ParseLine(line string, header []string, buffer []string) Record {
	parts := FastSplit(line, buffer)

	rec := Record{
		Dir: "T",
	}

	var props map[string]string

	for i := 0; i < len(header); i++ {
		val := ""
		if i < len(parts) {
			val = strings.TrimSpace(parts[i])
		}

		key := header[i]

		switch key {
		case "@id":
			rec.ID = val
		case "@label":
			rec.Label = val
		case "@out":
			rec.Src = val
		case "@in":
			rec.Dst = val
		case "@dir":
			if val != "" {
				rec.Dir = val
			}
		default:
			// if not special
			if !strings.HasPrefix(key, "@") && val != "" {
				if props == nil {
					props = make(map[string]string)
				}
				props[key] = val
			}
		}
	}

	if props == nil {
		props = make(map[string]string)
	}
	rec.Props = props

	return rec
}
