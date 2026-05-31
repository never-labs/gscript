package csv

import (
	stdcsv "encoding/csv"
	"io"
	"strings"
)

type Options struct {
	Sep        rune
	Comment    rune
	TrimSpace  bool
	LazyQuotes bool
}

func Parse(data string, opts Options) ([][]string, error) {
	r := stdcsv.NewReader(strings.NewReader(data))
	r.FieldsPerRecord = -1
	configureReader(r, opts)

	var rows [][]string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, append([]string(nil), record...))
	}
	return rows, nil
}

func ParseWithHeaders(data string, opts Options) ([]map[string]string, error) {
	r := stdcsv.NewReader(strings.NewReader(data))
	r.FieldsPerRecord = -1
	configureReader(r, opts)

	headers, err := r.Read()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		row := make(map[string]string, len(headers))
		for i, field := range record {
			if i < len(headers) {
				row[headers[i]] = field
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func Encode(rows [][]string, opts Options) (string, error) {
	var sb strings.Builder
	if err := Write(rows, opts, &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func Write(rows [][]string, opts Options, w io.Writer) error {
	cw := stdcsv.NewWriter(w)
	configureWriter(cw, opts)
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func EncodeWithHeaders(rows []map[string]string, headers []string, opts Options) (string, error) {
	var sb strings.Builder
	if err := WriteWithHeaders(rows, headers, opts, &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func WriteWithHeaders(rows []map[string]string, headers []string, opts Options, w io.Writer) error {
	cw := stdcsv.NewWriter(w)
	configureWriter(cw, opts)
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(headers))
		for i, header := range headers {
			record[i] = row[header]
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func configureReader(r *stdcsv.Reader, opts Options) {
	if opts.Sep != 0 {
		r.Comma = opts.Sep
	}
	if opts.Comment != 0 {
		r.Comment = opts.Comment
	}
	r.TrimLeadingSpace = opts.TrimSpace
	r.LazyQuotes = opts.LazyQuotes
}

func configureWriter(w *stdcsv.Writer, opts Options) {
	if opts.Sep != 0 {
		w.Comma = opts.Sep
	}
}
