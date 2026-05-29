package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	fileMarkerKey = "__gscript_file"
	fileClosedKey = "__gscript_file_closed"
)

type gscriptFileHandle struct {
	file   *os.File
	reader *bufio.Reader
	table  *Table
	std    bool
	closed bool
}

// buildIOLib creates the "io" standard library table.
func buildIOLib(interps ...*Interpreter) *Table {
	t := NewTable()
	var interp *Interpreter
	if len(interps) > 0 {
		interp = interps[0]
	}
	currentInput := newFileHandle(os.Stdin, true)
	currentOutput := newFileHandle(os.Stdout, true)
	stdErr := newFileHandle(os.Stderr, true)

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "io." + name,
			Fn:   fn,
		}))
	}

	// io.write(...) -- write to stdout (no newline)
	set("write", func(args []Value) ([]Value, error) {
		if currentOutput.closed {
			return []Value{NilValue(), StringValue("file is closed")}, nil
		}
		for _, a := range args {
			if _, err := currentOutput.file.WriteString(a.String()); err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
		}
		return nil, nil
	})

	// io.read([fmt]) -- read from stdin
	// "*l" (or "l") → read a line (default)
	// "*n" (or "n") → read a number
	// "*a" (or "a") → read all remaining input
	set("read", func(args []Value) ([]Value, error) {
		if currentInput.closed {
			return []Value{NilValue(), StringValue("file is closed")}, nil
		}
		return currentInput.read(args, 0)
	})

	// io.lines([filename]) -> iterator
	set("lines", func(args []Value) ([]Value, error) {
		var scanner *bufio.Scanner
		var opened *gscriptFileHandle

		if len(args) >= 1 && args[0].IsString() {
			var err error
			filename, err := resolveIOPath(interp, args[0].Str(), true, false)
			if err != nil {
				return nil, err
			}
			file, err := os.Open(filename)
			if err != nil {
				return nil, fmt.Errorf("cannot open '%s': %s", args[0].Str(), err)
			}
			opened = newFileHandle(file, false)
			scanner = bufio.NewScanner(opened.reader)
		} else {
			if currentInput.closed {
				return []Value{NilValue(), StringValue("file is closed")}, nil
			}
			scanner = bufio.NewScanner(currentInput.reader)
		}

		iter := &GoFunction{
			Name: "io.lines_iterator",
			Fn: func(_ []Value) ([]Value, error) {
				if scanner.Scan() {
					return []Value{StringValue(scanner.Text())}, nil
				}
				if opened != nil {
					_ = opened.close()
				}
				if err := scanner.Err(); err != nil {
					return nil, err
				}
				return []Value{NilValue()}, nil
			},
		}
		return []Value{FunctionValue(iter)}, nil
	})

	// io.open(filename, mode) -> file table, err
	set("open", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'io.open' (string expected)")
		}
		filename := args[0].Str()
		mode := "r"
		if len(args) >= 2 && args[1].IsString() {
			mode = args[1].Str()
		}

		flag, ok := parseFileMode(mode)
		if !ok {
			return []Value{NilValue(), StringValue(fmt.Sprintf("invalid mode: %s", mode))}, nil
		}

		read, write := fileModeAccess(flag)
		resolved, err := resolveIOPath(interp, filename, read, write)
		if err != nil {
			return nil, err
		}
		file, err := os.OpenFile(resolved, flag, 0644)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}

		return []Value{TableValue(newFileHandle(file, false).table)}, nil
	})

	// io.flush() -- flush the current output stream.
	set("flush", func(args []Value) ([]Value, error) {
		if currentOutput.closed {
			return []Value{NilValue(), StringValue("file is closed")}, nil
		}
		if err := currentOutput.flush(); err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// io.input([file|string]) -> current input file, or sets it.
	set("input", func(args []Value) ([]Value, error) {
		if len(args) == 0 {
			return []Value{TableValue(currentInput.table)}, nil
		}
		h, err := inputOutputTarget(interp, args[0], os.O_RDONLY)
		if err != nil {
			if strings.Contains(err.Error(), "filesystem ") {
				return nil, err
			}
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		currentInput = h
		return []Value{TableValue(currentInput.table)}, nil
	})

	// io.output([file|string]) -> current output file, or sets it.
	set("output", func(args []Value) ([]Value, error) {
		if len(args) == 0 {
			return []Value{TableValue(currentOutput.table)}, nil
		}
		h, err := inputOutputTarget(interp, args[0], os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			if strings.Contains(err.Error(), "filesystem ") {
				return nil, err
			}
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		currentOutput = h
		return []Value{TableValue(currentOutput.table)}, nil
	})

	// io.type(obj) -> "file", "closed file", or nil.
	set("type", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'io.type' (value expected)")
		}
		if !args[0].IsTable() || !isFileTable(args[0].Table()) {
			return []Value{NilValue()}, nil
		}
		if args[0].Table().RawGet(StringValue(fileClosedKey)).Truthy() {
			return []Value{StringValue("closed file")}, nil
		}
		return []Value{StringValue("file")}, nil
	})

	// io.tmpfile() -> temporary read/write file handle.
	set("tmpfile", func(args []Value) ([]Value, error) {
		if interp != nil && !interp.filesystemWrite {
			return nil, fmt.Errorf("filesystem write access disabled")
		}
		dir := ""
		if interp != nil && interp.filesystemRoot != "" {
			dir = interp.filesystemRoot
		}
		file, err := os.CreateTemp(dir, "gscript-*")
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		_ = os.Remove(file.Name())
		return []Value{TableValue(newFileHandle(file, false).table)}, nil
	})

	t.RawSet(StringValue("stdin"), TableValue(currentInput.table))
	t.RawSet(StringValue("stdout"), TableValue(currentOutput.table))
	t.RawSet(StringValue("stderr"), TableValue(stdErr.table))

	return t
}

func (interp *Interpreter) refreshIOLib() {
	if v, ok := interp.globals.Get("io"); ok && v.IsTable() {
		ioLib := TableValue(buildIOLib(interp))
		interp.globals.Define("io", ioLib)
		interp.modules["io"] = ioLib
		interp.markPackageLoaded("io", ioLib)
	}
}

func parseFileMode(mode string) (int, bool) {
	mode = strings.ReplaceAll(mode, "b", "")
	switch mode {
	case "r":
		return os.O_RDONLY, true
	case "w":
		return os.O_WRONLY | os.O_CREATE | os.O_TRUNC, true
	case "a":
		return os.O_WRONLY | os.O_CREATE | os.O_APPEND, true
	case "r+":
		return os.O_RDWR, true
	case "w+":
		return os.O_RDWR | os.O_CREATE | os.O_TRUNC, true
	case "a+":
		return os.O_RDWR | os.O_CREATE | os.O_APPEND, true
	default:
		return 0, false
	}
}

func fileModeAccess(flag int) (read, write bool) {
	read = flag&os.O_WRONLY == 0 || flag&os.O_RDWR != 0
	write = flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0
	return read, write
}

func resolveIOPath(interp *Interpreter, path string, read, write bool) (string, error) {
	if interp == nil {
		return path, nil
	}
	if read && !interp.filesystemRead {
		return "", fmt.Errorf("filesystem read access disabled")
	}
	if write && !interp.filesystemWrite {
		return "", fmt.Errorf("filesystem write access disabled")
	}
	return resolveSandboxPath(interp.filesystemRoot, path)
}

func inputOutputTarget(interp *Interpreter, v Value, stringMode int) (*gscriptFileHandle, error) {
	if v.IsString() {
		read, write := fileModeAccess(stringMode)
		path, err := resolveIOPath(interp, v.Str(), read, write)
		if err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, stringMode, 0644)
		if err != nil {
			return nil, err
		}
		return newFileHandle(file, false), nil
	}
	if v.IsTable() && isFileTable(v.Table()) {
		fileHandleRegistry.RLock()
		h, ok := fileHandleRegistry.handles[v.Table()]
		fileHandleRegistry.RUnlock()
		if !ok {
			return nil, fmt.Errorf("invalid file handle")
		}
		if h.closed {
			return nil, fmt.Errorf("file is closed")
		}
		return h, nil
	}
	return nil, fmt.Errorf("file handle or filename expected")
}

var fileHandleRegistry = struct {
	sync.RWMutex
	handles map[*Table]*gscriptFileHandle
}{handles: map[*Table]*gscriptFileHandle{}}

func isFileTable(t *Table) bool {
	return t.RawGet(StringValue(fileMarkerKey)).Truthy()
}

func newFileHandle(file *os.File, std bool) *gscriptFileHandle {
	h := &gscriptFileHandle{
		file:   file,
		reader: bufio.NewReader(file),
		std:    std,
	}
	h.table = buildFileTable(h)
	fileHandleRegistry.Lock()
	fileHandleRegistry.handles[h.table] = h
	fileHandleRegistry.Unlock()
	return h
}

func (h *gscriptFileHandle) markClosed() {
	h.closed = true
	if h.table != nil {
		h.table.RawSet(StringValue(fileClosedKey), BoolValue(true))
	}
}

func (h *gscriptFileHandle) close() error {
	if h.closed {
		return fmt.Errorf("file is already closed")
	}
	if h.std {
		h.markClosed()
		return nil
	}
	err := h.file.Close()
	if err == nil {
		h.markClosed()
	}
	return err
}

func (h *gscriptFileHandle) flush() error {
	if h.closed {
		return fmt.Errorf("file is closed")
	}
	if h.std {
		return nil
	}
	return h.file.Sync()
}

func (h *gscriptFileHandle) seek(whence string, offset int64) (int64, error) {
	if h.closed {
		return 0, fmt.Errorf("file is closed")
	}
	var base int
	switch whence {
	case "set":
		base = io.SeekStart
	case "cur":
		base = io.SeekCurrent
	case "end":
		base = io.SeekEnd
	default:
		return 0, fmt.Errorf("invalid whence: %s", whence)
	}
	pos, err := h.file.Seek(offset, base)
	if err != nil {
		return 0, err
	}
	h.reader.Reset(h.file)
	return pos, nil
}

func (h *gscriptFileHandle) read(args []Value, start int) ([]Value, error) {
	if h.closed {
		return []Value{NilValue(), StringValue("file is closed")}, nil
	}
	if len(args) <= start {
		v, err := h.readOne(StringValue("*l"))
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{v}, nil
	}
	results := make([]Value, 0, len(args)-start)
	for _, format := range args[start:] {
		v, err := h.readOne(format)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		results = append(results, v)
	}
	return results, nil
}

func (h *gscriptFileHandle) readOne(format Value) (Value, error) {
	if format.IsNumber() {
		n := int(toInt(format))
		if n < 0 {
			return NilValue(), fmt.Errorf("invalid read count: %d", n)
		}
		if n == 0 {
			if _, err := h.reader.Peek(1); err == io.EOF {
				return NilValue(), nil
			} else if err != nil {
				return NilValue(), err
			}
			return StringValue(""), nil
		}
		buf := make([]byte, n)
		read, err := io.ReadFull(h.reader, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return NilValue(), err
		}
		if read == 0 && err == io.EOF {
			return NilValue(), nil
		}
		return StringValue(string(buf[:read])), nil
	}
	if !format.IsString() {
		return NilValue(), fmt.Errorf("invalid read format")
	}
	switch fmtStr := format.Str(); fmtStr {
	case "*l", "l":
		line, err := h.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return NilValue(), err
		}
		if len(line) == 0 && err == io.EOF {
			return NilValue(), nil
		}
		return StringValue(strings.TrimRight(line, "\n\r")), nil
	case "*L", "L":
		line, err := h.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return NilValue(), err
		}
		if len(line) == 0 && err == io.EOF {
			return NilValue(), nil
		}
		return StringValue(line), nil
	case "*a", "a":
		data, err := io.ReadAll(h.reader)
		if err != nil {
			return NilValue(), err
		}
		return StringValue(string(data)), nil
	case "*n", "n":
		line, err := h.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return NilValue(), err
		}
		line = strings.TrimSpace(line)
		if i, err := strconv.ParseInt(line, 10, 64); err == nil {
			return IntValue(i), nil
		}
		if f, err := strconv.ParseFloat(line, 64); err == nil {
			return FloatValue(f), nil
		}
		return NilValue(), nil
	default:
		return NilValue(), fmt.Errorf("invalid format: %s", fmtStr)
	}
}

// buildFileTable creates a table representing a file object with read/write/close/lines methods.
func buildFileTable(h *gscriptFileHandle) *Table {
	ft := NewTable()
	ft.RawSet(StringValue(fileMarkerKey), BoolValue(true))
	ft.RawSet(StringValue(fileClosedKey), BoolValue(false))

	ft.RawSet(StringValue("read"), FunctionValue(&GoFunction{
		Name: "file:read",
		Fn: func(args []Value) ([]Value, error) {
			return h.read(args, 1)
		},
	}))

	ft.RawSet(StringValue("write"), FunctionValue(&GoFunction{
		Name: "file:write",
		Fn: func(args []Value) ([]Value, error) {
			if h.closed {
				return []Value{NilValue(), StringValue("file is closed")}, nil
			}
			for _, a := range args[1:] {
				_, err := h.file.WriteString(a.String())
				if err != nil {
					return []Value{NilValue(), StringValue(err.Error())}, nil
				}
			}
			return []Value{TableValue(ft)}, nil
		},
	}))

	ft.RawSet(StringValue("close"), FunctionValue(&GoFunction{
		Name: "file:close",
		Fn: func(args []Value) ([]Value, error) {
			if err := h.close(); err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{BoolValue(true)}, nil
		},
	}))

	ft.RawSet(StringValue("flush"), FunctionValue(&GoFunction{
		Name: "file:flush",
		Fn: func(args []Value) ([]Value, error) {
			if err := h.flush(); err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{BoolValue(true)}, nil
		},
	}))

	ft.RawSet(StringValue("seek"), FunctionValue(&GoFunction{
		Name: "file:seek",
		Fn: func(args []Value) ([]Value, error) {
			whence := "cur"
			if len(args) >= 2 && args[1].IsString() {
				whence = args[1].Str()
			}
			var offset int64
			if len(args) >= 3 {
				offset = toInt(args[2])
			}
			pos, err := h.seek(whence, offset)
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{IntValue(pos)}, nil
		},
	}))

	ft.RawSet(StringValue("lines"), FunctionValue(&GoFunction{
		Name: "file:lines",
		Fn: func(args []Value) ([]Value, error) {
			if h.closed {
				return []Value{NilValue(), StringValue("file is closed")}, nil
			}
			scanner := bufio.NewScanner(h.reader)
			iter := &GoFunction{
				Name: "file:lines_iterator",
				Fn: func(_ []Value) ([]Value, error) {
					if scanner.Scan() {
						return []Value{StringValue(scanner.Text())}, nil
					}
					if err := scanner.Err(); err != nil {
						return nil, err
					}
					return []Value{NilValue()}, nil
				},
			}
			return []Value{FunctionValue(iter)}, nil
		},
	}))

	return ft
}
