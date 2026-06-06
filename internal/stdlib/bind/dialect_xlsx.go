package bind

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func dialectXLSX(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := dialectMode(opts)
	if mode == "" {
		if body.IsTable() {
			mode = "encode"
		} else {
			mode = "decode"
		}
	}
	switch mode {
	case "", "decode", "parse":
		if !body.IsString() {
			return nil, fmt.Errorf("xlsx dialect: decode expects string bytes")
		}
		rows, err := xlsxDecodeRows([]byte(body.Str()), hostResultLimit(maxHostResult))
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		if opts != nil && opts.RawGetString("headers").Truthy() {
			return []Value{xlsxHeaderRowsToValue(rows)}, nil
		}
		return []Value{xlsxRowsToValue(rows)}, nil
	case "encode", "format":
		if !body.IsTable() {
			return nil, fmt.Errorf("xlsx dialect: encode expects table body")
		}
		rows, err := xlsxRowsFromValue(body, opts)
		if err != nil {
			return nil, err
		}
		sheet := "Sheet1"
		if opts != nil && opts.RawGetString("sheet").IsString() && opts.RawGetString("sheet").Str() != "" {
			sheet = opts.RawGetString("sheet").Str()
		}
		out, err := xlsxEncodeRows(rows, sheet, hostResultLimit(maxHostResult))
		if err != nil {
			return nil, err
		}
		return []Value{StringValue(string(out))}, nil
	default:
		return dialectUnknownMode("xlsx", mode)
	}
}

func xlsxRowsToValue(rows [][]string) Value {
	out := NewAppendArrayTable(len(rows))
	for i, row := range rows {
		rowTbl := NewAppendArrayTable(len(row))
		for j, cell := range row {
			rowTbl.RawSetInt(int64(j+1), StringValue(cell))
		}
		out.RawSetInt(int64(i+1), TableValue(rowTbl))
	}
	return TableValue(out)
}

func xlsxHeaderRowsToValue(rows [][]string) Value {
	if len(rows) == 0 {
		return TableValue(NewAppendArrayTable(0))
	}
	headers := rows[0]
	out := NewAppendArrayTable(maxInt(0, len(rows)-1))
	for i, row := range rows[1:] {
		rowTbl := NewTableSized(0, len(headers))
		for j, header := range headers {
			if header == "" {
				continue
			}
			value := ""
			if j < len(row) {
				value = row[j]
			}
			rowTbl.RawSetString(header, StringValue(value))
		}
		out.RawSetInt(int64(i+1), TableValue(rowTbl))
	}
	return TableValue(out)
}

func xlsxRowsFromValue(rowsVal Value, opts *Table) ([][]string, error) {
	if opts != nil && opts.RawGetString("headers").IsTable() {
		headers := xlsxStringSliceFromTable(opts.RawGetString("headers").Table())
		rows := [][]string{headers}
		body := rowsVal.Table()
		for i := int64(1); i <= int64(body.Length()); i++ {
			rowVal := body.RawGetInt(i)
			if !rowVal.IsTable() {
				continue
			}
			rowTbl := rowVal.Table()
			record := make([]string, len(headers))
			for j, header := range headers {
				record[j] = rowTbl.RawGetString(header).String()
			}
			rows = append(rows, record)
		}
		return rows, nil
	}
	body := rowsVal.Table()
	rows := make([][]string, 0, body.Length())
	for i := int64(1); i <= int64(body.Length()); i++ {
		rowVal := body.RawGetInt(i)
		if !rowVal.IsTable() {
			continue
		}
		rowTbl := rowVal.Table()
		record := make([]string, rowTbl.Length())
		for j := int64(1); j <= int64(rowTbl.Length()); j++ {
			record[j-1] = rowTbl.RawGetInt(j).String()
		}
		rows = append(rows, record)
	}
	return rows, nil
}

func xlsxStringSliceFromTable(tbl *Table) []string {
	out := make([]string, 0, tbl.Length())
	for i := int64(1); i <= int64(tbl.Length()); i++ {
		out = append(out, tbl.RawGetInt(i).String())
	}
	return out
}

func xlsxEncodeRows(rows [][]string, sheetName string, limit int64) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypesXML,
		"_rels/.rels":                xlsxRootRelsXML,
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRelsXML,
		"xl/styles.xml":              xlsxStylesXML,
		"xl/workbook.xml":            xlsxWorkbookXML(sheetName),
		"xl/worksheets/sheet1.xml":   xlsxWorksheetXML(rows),
		"docProps/app.xml":           xlsxAppXML,
		"docProps/core.xml":          xlsxCoreXML,
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := CheckProjectedHostStringBytes(limit, buf.Len()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func xlsxWorksheetXML(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r, row := range rows {
		rowNum := r + 1
		b.WriteString(`<row r="`)
		b.WriteString(strconv.Itoa(rowNum))
		b.WriteString(`">`)
		for c, cell := range row {
			ref := xlsxCellRef(c+1, rowNum)
			b.WriteString(`<c r="`)
			b.WriteString(ref)
			if _, err := strconv.ParseFloat(cell, 64); err == nil && strings.TrimSpace(cell) != "" {
				b.WriteString(`"><v>`)
				b.WriteString(xmlEscape(cell))
				b.WriteString(`</v></c>`)
			} else {
				b.WriteString(`" t="inlineStr"><is><t>`)
				b.WriteString(xmlEscape(cell))
				b.WriteString(`</t></is></c>`)
			}
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func xlsxCellRef(col, row int) string {
	var letters []byte
	for col > 0 {
		col--
		letters = append([]byte{byte('A' + col%26)}, letters...)
		col /= 26
	}
	return string(letters) + strconv.Itoa(row)
}

func xlsxWorkbookXML(sheetName string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="` + xmlEscape(sheetName) + `" sheetId="1" r:id="rId1"/></sheets></workbook>`
}

func xlsxDecodeRows(data []byte, limit int64) ([][]string, error) {
	if err := CheckProjectedHostStringBytes(limit, len(data)); err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xlsx dialect: open zip: %w", err)
	}
	shared, err := xlsxSharedStrings(zr, limit)
	if err != nil {
		return nil, err
	}
	sheetFile := xlsxFirstWorksheet(zr)
	if sheetFile == nil {
		return nil, fmt.Errorf("xlsx dialect: worksheet not found")
	}
	rc, err := sheetFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	xmlData, err := ReadAllWithHostResultLimit(rc, limit)
	if err != nil {
		return nil, err
	}
	var sheet xlsxWorksheet
	if err := xml.Unmarshal(xmlData, &sheet); err != nil {
		return nil, fmt.Errorf("xlsx dialect: parse worksheet: %w", err)
	}
	rows := make([][]string, 0, len(sheet.SheetData.Rows))
	for _, row := range sheet.SheetData.Rows {
		record := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			text := xlsxCellText(cell, shared)
			col := xlsxColumnIndex(cell.Ref)
			if col <= 0 {
				record = append(record, text)
				continue
			}
			for len(record) < col {
				record = append(record, "")
			}
			record[col-1] = text
		}
		rows = append(rows, record)
	}
	return rows, nil
}

func xlsxFirstWorksheet(zr *zip.Reader) *zip.File {
	var first *zip.File
	firstIndex := int(^uint(0) >> 1)
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			index := xlsxWorksheetIndex(f.Name)
			if first == nil || index < firstIndex || (index == firstIndex && f.Name < first.Name) {
				first = f
				firstIndex = index
			}
		}
	}
	return first
}

func xlsxWorksheetIndex(name string) int {
	base := strings.TrimPrefix(name, "xl/worksheets/sheet")
	base = strings.TrimSuffix(base, ".xml")
	index, err := strconv.Atoi(base)
	if err != nil || index <= 0 {
		return int(^uint(0) >> 1)
	}
	return index
}

func xlsxSharedStrings(zr *zip.Reader, limit int64) ([]string, error) {
	var file *zip.File
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			file = f
			break
		}
	}
	if file == nil {
		return nil, nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := ReadAllWithHostResultLimit(rc, limit)
	if err != nil {
		return nil, err
	}
	var doc xlsxSharedStringTable
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("xlsx dialect: parse shared strings: %w", err)
	}
	out := make([]string, 0, len(doc.Items))
	for _, item := range doc.Items {
		var text strings.Builder
		text.WriteString(item.Text)
		for _, run := range item.Runs {
			text.WriteString(run.Text)
		}
		out = append(out, text.String())
	}
	return out, nil
}

func xlsxCellText(cell xlsxCell, shared []string) string {
	switch cell.Type {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return ""
	case "inlineStr":
		if cell.Inline.Text != "" {
			return cell.Inline.Text
		}
		var b strings.Builder
		for _, run := range cell.Inline.Runs {
			b.WriteString(run.Text)
		}
		return b.String()
	default:
		return cell.Value
	}
}

func xlsxColumnIndex(ref string) int {
	col := 0
	for i := 0; i < len(ref); i++ {
		ch := ref[i]
		switch {
		case ch >= 'A' && ch <= 'Z':
			col = col*26 + int(ch-'A'+1)
		case ch >= 'a' && ch <= 'z':
			col = col*26 + int(ch-'a'+1)
		default:
			if col == 0 {
				return 0
			}
			return col
		}
	}
	return col
}

type xlsxWorksheet struct {
	SheetData struct {
		Rows []xlsxRow `xml:"row"`
	} `xml:"sheetData"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Ref    string         `xml:"r,attr"`
	Type   string         `xml:"t,attr"`
	Value  string         `xml:"v"`
	Inline xlsxInlineText `xml:"is"`
}

type xlsxInlineText struct {
	Text string    `xml:"t"`
	Runs []xlsxRun `xml:"r"`
}

type xlsxSharedStringTable struct {
	Items []xlsxSharedString `xml:"si"`
}

type xlsxSharedString struct {
	Text string    `xml:"t"`
	Runs []xlsxRun `xml:"r"`
}

type xlsxRun struct {
	Text string `xml:"t"`
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const xlsxContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`

const xlsxRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

const xlsxWorkbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const xlsxStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellXfs></styleSheet>`

const xlsxAppXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>Leia</Application></Properties>`

const xlsxCoreXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>Leia</dc:creator></cp:coreProperties>`
