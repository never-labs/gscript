package io

import (
	"bufio"
	"fmt"
	stdio "io"
	"strconv"
	"strings"

	"github.com/never-labs/gscript/internal/hostpath"
)

type ReadFormat struct {
	count   int
	isCount bool
	format  string
}

func CountFormat(n int) ReadFormat {
	return ReadFormat{count: n, isCount: true}
}

func StringFormat(format string) ReadFormat {
	return ReadFormat{format: format}
}

type ReadKind int

const (
	ReadNil ReadKind = iota
	ReadString
	ReadInt
	ReadFloat
)

type ReadResult struct {
	Kind   ReadKind
	String string
	Int    int64
	Float  float64
}

func ResolvePath(root, path string, readEnabled, writeEnabled, read, write bool) (string, error) {
	if read && !readEnabled {
		return "", fmt.Errorf("filesystem read access disabled")
	}
	if write && !writeEnabled {
		return "", fmt.Errorf("filesystem write access disabled")
	}
	return hostpath.ResolveSandboxPath(root, path)
}

func SeekWhence(whence string) (int, error) {
	switch whence {
	case "set":
		return stdio.SeekStart, nil
	case "cur":
		return stdio.SeekCurrent, nil
	case "end":
		return stdio.SeekEnd, nil
	default:
		return 0, fmt.Errorf("invalid whence: %s", whence)
	}
}

func ReadOne(reader *bufio.Reader, format ReadFormat) (ReadResult, error) {
	if format.isCount {
		n := format.count
		if n < 0 {
			return ReadResult{Kind: ReadNil}, fmt.Errorf("invalid read count: %d", n)
		}
		if n == 0 {
			if _, err := reader.Peek(1); err == stdio.EOF {
				return ReadResult{Kind: ReadNil}, nil
			} else if err != nil {
				return ReadResult{Kind: ReadNil}, err
			}
			return ReadResult{Kind: ReadString, String: ""}, nil
		}
		buf := make([]byte, n)
		read, err := stdio.ReadFull(reader, buf)
		if err != nil && err != stdio.ErrUnexpectedEOF && err != stdio.EOF {
			return ReadResult{Kind: ReadNil}, err
		}
		if read == 0 && err == stdio.EOF {
			return ReadResult{Kind: ReadNil}, nil
		}
		return ReadResult{Kind: ReadString, String: string(buf[:read])}, nil
	}
	switch fmtStr := format.format; fmtStr {
	case "*l", "l":
		line, err := reader.ReadString('\n')
		if err != nil && err != stdio.EOF {
			return ReadResult{Kind: ReadNil}, err
		}
		if len(line) == 0 && err == stdio.EOF {
			return ReadResult{Kind: ReadNil}, nil
		}
		return ReadResult{Kind: ReadString, String: strings.TrimRight(line, "\n\r")}, nil
	case "*L", "L":
		line, err := reader.ReadString('\n')
		if err != nil && err != stdio.EOF {
			return ReadResult{Kind: ReadNil}, err
		}
		if len(line) == 0 && err == stdio.EOF {
			return ReadResult{Kind: ReadNil}, nil
		}
		return ReadResult{Kind: ReadString, String: line}, nil
	case "*a", "a":
		data, err := stdio.ReadAll(reader)
		if err != nil {
			return ReadResult{Kind: ReadNil}, err
		}
		return ReadResult{Kind: ReadString, String: string(data)}, nil
	case "*n", "n":
		line, err := reader.ReadString('\n')
		if err != nil && err != stdio.EOF {
			return ReadResult{Kind: ReadNil}, err
		}
		line = strings.TrimSpace(line)
		if i, err := strconv.ParseInt(line, 10, 64); err == nil {
			return ReadResult{Kind: ReadInt, Int: i}, nil
		}
		if f, err := strconv.ParseFloat(line, 64); err == nil {
			return ReadResult{Kind: ReadFloat, Float: f}, nil
		}
		return ReadResult{Kind: ReadNil}, nil
	default:
		return ReadResult{Kind: ReadNil}, fmt.Errorf("invalid format: %s", fmtStr)
	}
}
