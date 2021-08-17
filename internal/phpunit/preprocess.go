package phpunit

import (
	"bytes"
	"sort"
)

type textEdit struct {
	StartPos int
	EndPos   int

	Replacement string
}

func applyTextEdits(contents []byte, fixes []textEdit) []byte {
	if len(fixes) == 0 {
		return nil
	}

	sort.Slice(fixes, func(i, j int) bool {
		return fixes[i].StartPos < fixes[j].StartPos
	})

	var buf bytes.Buffer
	buf.Grow(len(contents))
	writeTextEdits(&buf, contents, fixes)

	return buf.Bytes()
}

func writeTextEdits(buf *bytes.Buffer, contents []byte, fixes []textEdit) {
	offset := 0
	for _, fix := range fixes {
		// If we have a nested replacement, apply only outer replacement.
		if offset > fix.StartPos {
			continue
		}

		buf.Write(contents[offset:fix.StartPos])
		buf.WriteString(fix.Replacement)

		offset = fix.EndPos
	}
	buf.Write(contents[offset:])
}
