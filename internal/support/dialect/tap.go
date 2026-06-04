package dialect

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type TAPDocument struct {
	Rows []TAPRow
}

type TAPRow struct {
	Kind        string
	Line        int
	Version     int
	OK          bool
	Number      int
	Name        string
	Directive   string
	Reason      string
	First       int
	Last        int
	Text        string
	Diagnostics []string
}

var (
	tapVersionRe = regexp.MustCompile(`^TAP\s+version\s+([0-9]+)$`)
	tapPlanRe    = regexp.MustCompile(`^([0-9]+)\.\.([0-9]+)(?:\s*#\s*(.*))?$`)
	tapTestRe    = regexp.MustCompile(`^(not\s+ok|ok)(?:\s+([0-9]+))?(?:\s*-\s*(.*?))?(?:\s+#\s*(.*))?$`)
)

func ParseTAP(src string) (TAPDocument, error) {
	lines := Lines(src, true, false)
	doc := TAPDocument{Rows: make([]TAPRow, 0, len(lines))}
	lastTest := -1
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			text := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if lastTest >= 0 {
				doc.Rows[lastTest].Diagnostics = append(doc.Rows[lastTest].Diagnostics, text)
				continue
			}
			doc.Rows = append(doc.Rows, TAPRow{Kind: "diagnostic", Line: lineNo, Text: text})
			continue
		}
		if m := tapVersionRe.FindStringSubmatch(line); m != nil {
			version, _ := strconv.Atoi(m[1])
			doc.Rows = append(doc.Rows, TAPRow{Kind: "version", Line: lineNo, Version: version})
			continue
		}
		if m := tapPlanRe.FindStringSubmatch(line); m != nil {
			first, _ := strconv.Atoi(m[1])
			last, _ := strconv.Atoi(m[2])
			row := TAPRow{Kind: "plan", Line: lineNo, First: first, Last: last}
			row.Directive, row.Reason = parseTAPDirective(m[3])
			doc.Rows = append(doc.Rows, row)
			continue
		}
		if m := tapTestRe.FindStringSubmatch(line); m != nil {
			row := TAPRow{
				Kind: "test",
				Line: lineNo,
				OK:   m[1] == "ok",
				Name: strings.TrimSpace(m[3]),
			}
			if m[2] != "" {
				row.Number, _ = strconv.Atoi(m[2])
			}
			row.Directive, row.Reason = parseTAPDirective(m[4])
			doc.Rows = append(doc.Rows, row)
			lastTest = len(doc.Rows) - 1
			continue
		}
		return TAPDocument{}, &ParseError{Kind: "tap", Message: fmt.Sprintf("line %d: unrecognized TAP line %q", lineNo, raw)}
	}
	return doc, nil
}

func EncodeTAP(doc TAPDocument) (string, error) {
	var b strings.Builder
	for i, row := range doc.Rows {
		switch row.Kind {
		case "version":
			version := row.Version
			if version == 0 {
				version = 13
			}
			fmt.Fprintf(&b, "TAP version %d\n", version)
		case "plan":
			first, last := row.First, row.Last
			if first == 0 {
				first = 1
			}
			fmt.Fprintf(&b, "%d..%d", first, last)
			writeTAPDirective(&b, row.Directive, row.Reason)
			b.WriteByte('\n')
		case "test":
			if row.OK {
				b.WriteString("ok")
			} else {
				b.WriteString("not ok")
			}
			if row.Number > 0 {
				fmt.Fprintf(&b, " %d", row.Number)
			}
			if row.Name != "" {
				b.WriteString(" - ")
				b.WriteString(row.Name)
			}
			writeTAPDirective(&b, row.Directive, row.Reason)
			b.WriteByte('\n')
			for _, diagnostic := range row.Diagnostics {
				b.WriteString("# ")
				b.WriteString(diagnostic)
				b.WriteByte('\n')
			}
		case "diagnostic":
			b.WriteString("# ")
			b.WriteString(row.Text)
			b.WriteByte('\n')
		default:
			return "", &ParseError{Kind: "tap", Message: fmt.Sprintf("row %d: unsupported kind %q", i+1, row.Kind)}
		}
	}
	return b.String(), nil
}

func parseTAPDirective(src string) (string, string) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", ""
	}
	parts := strings.Fields(src)
	if len(parts) == 0 {
		return "", ""
	}
	directive := strings.ToUpper(parts[0])
	if directive != "SKIP" && directive != "TODO" {
		return "", src
	}
	reason := strings.TrimSpace(strings.TrimPrefix(src, parts[0]))
	return directive, reason
}

func writeTAPDirective(b *strings.Builder, directive, reason string) {
	directive = strings.ToUpper(strings.TrimSpace(directive))
	reason = strings.TrimSpace(reason)
	if directive == "" && reason == "" {
		return
	}
	b.WriteString(" #")
	if directive != "" {
		b.WriteByte(' ')
		b.WriteString(directive)
	}
	if reason != "" {
		b.WriteByte(' ')
		b.WriteString(reason)
	}
}
