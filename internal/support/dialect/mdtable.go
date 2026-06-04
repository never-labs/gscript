package dialect

import (
	"fmt"
	"strings"
)

type MarkdownTable struct {
	Headers []string
	Rows    []map[string]string
}

func ParseMarkdownTable(src string) (MarkdownTable, error) {
	lines := Lines(src, true, false)
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if end-start < 2 {
		return MarkdownTable{}, &ParseError{Kind: "mdtable", Message: "expected header and delimiter rows"}
	}

	headers := splitMarkdownTableRow(lines[start])
	if len(headers) == 0 {
		return MarkdownTable{}, &ParseError{Kind: "mdtable", Message: "header row has no columns"}
	}
	if err := validateMarkdownDelimiter(lines[start+1], len(headers)); err != nil {
		return MarkdownTable{}, err
	}

	rows := make([]map[string]string, 0, end-start-2)
	for lineNo := start + 2; lineNo < end; lineNo++ {
		line := lines[lineNo]
		if strings.TrimSpace(line) == "" {
			return MarkdownTable{}, &ParseError{Kind: "mdtable", Message: fmt.Sprintf("line %d: blank row inside table", lineNo+1)}
		}
		cells := splitMarkdownTableRow(line)
		row := make(map[string]string, len(headers))
		for i, header := range headers {
			if i < len(cells) {
				row[header] = cells[i]
			} else {
				row[header] = ""
			}
		}
		rows = append(rows, row)
	}
	return MarkdownTable{Headers: headers, Rows: rows}, nil
}

func EncodeMarkdownTable(table MarkdownTable) (string, error) {
	if len(table.Headers) == 0 {
		return "", &ParseError{Kind: "mdtable", Message: "headers are required for encode"}
	}
	var b strings.Builder
	writeMarkdownTableRow(&b, table.Headers)
	delimiter := make([]string, len(table.Headers))
	for i := range delimiter {
		delimiter[i] = "---"
	}
	writeMarkdownTableRow(&b, delimiter)
	for _, row := range table.Rows {
		cells := make([]string, len(table.Headers))
		for i, header := range table.Headers {
			cells[i] = row[header]
		}
		writeMarkdownTableRow(&b, cells)
	}
	return b.String(), nil
}

func validateMarkdownDelimiter(line string, want int) error {
	cells := splitMarkdownDelimiterRow(line)
	if len(cells) != want {
		return &ParseError{Kind: "mdtable", Message: fmt.Sprintf("delimiter row has %d columns, want %d", len(cells), want)}
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if len(cell) < 3 {
			return &ParseError{Kind: "mdtable", Message: "delimiter cells must contain at least three hyphens"}
		}
		if cell[0] == ':' {
			cell = cell[1:]
		}
		if len(cell) > 0 && cell[len(cell)-1] == ':' {
			cell = cell[:len(cell)-1]
		}
		if len(cell) < 3 {
			return &ParseError{Kind: "mdtable", Message: "delimiter cells must contain at least three hyphens"}
		}
		for i := 0; i < len(cell); i++ {
			if cell[i] != '-' {
				return &ParseError{Kind: "mdtable", Message: "delimiter cells may only contain hyphens and edge colons"}
			}
		}
	}
	return nil
}

func splitMarkdownTableRow(line string) []string {
	raw := splitMarkdownDelimiterRow(line)
	for i, cell := range raw {
		raw[i] = unescapeMarkdownTableCell(strings.TrimSpace(cell))
	}
	return raw
}

func splitMarkdownDelimiterRow(line string) []string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") && !endsWithEscapedPipe(line) {
		line = line[:len(line)-1]
	}
	var cells []string
	var cell strings.Builder
	escaped := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			cell.WriteByte('\\')
			cell.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '|' {
			cells = append(cells, cell.String())
			cell.Reset()
			continue
		}
		cell.WriteByte(ch)
	}
	if escaped {
		cell.WriteByte('\\')
	}
	cells = append(cells, cell.String())
	return cells
}

func endsWithEscapedPipe(line string) bool {
	if len(line) == 0 || line[len(line)-1] != '|' {
		return false
	}
	slashes := 0
	for i := len(line) - 2; i >= 0 && line[i] == '\\'; i-- {
		slashes++
	}
	return slashes%2 == 1
}

func unescapeMarkdownTableCell(cell string) string {
	var b strings.Builder
	escaped := false
	for i := 0; i < len(cell); i++ {
		ch := cell[i]
		if escaped {
			if ch == '|' || ch == '\\' {
				b.WriteByte(ch)
			} else {
				b.WriteByte('\\')
				b.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		b.WriteByte(ch)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func writeMarkdownTableRow(b *strings.Builder, cells []string) {
	b.WriteString("|")
	for _, cell := range cells {
		b.WriteByte(' ')
		b.WriteString(escapeMarkdownTableCell(cell))
		b.WriteString(" |")
	}
	b.WriteByte('\n')
}

func escapeMarkdownTableCell(cell string) string {
	cell = strings.ReplaceAll(cell, "\r\n", "<br>")
	cell = strings.ReplaceAll(cell, "\r", "<br>")
	cell = strings.ReplaceAll(cell, "\n", "<br>")
	cell = strings.ReplaceAll(cell, "\\", "\\\\")
	cell = strings.ReplaceAll(cell, "|", "\\|")
	return cell
}
