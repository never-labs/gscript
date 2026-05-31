package modules

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/never-labs/gscript/internal/hostpath"
	hostfs "github.com/never-labs/gscript/internal/stdlib/fs"
)

func resolveSandboxPath(root, path string) (string, error) {
	return hostpath.ResolveSandboxPath(root, path)
}

// BuildFS creates the "fs" standard library table.
func BuildFS(roots ...string) *Table {
	root := ""
	if len(roots) > 0 {
		root = roots[0]
	}
	return BuildFSWithPolicy(HostOptions{
		FilesystemRoot:  func() string { return root },
		FilesystemRead:  func() bool { return true },
		FilesystemWrite: func() bool { return true },
	})
}

func BuildFSWithPolicy(opts HostOptions) *Table {
	t := markStdlibrtModule(NewTable())
	root := func() string { return hostString(opts.FilesystemRoot) }
	read := func() bool { return hostBool(opts.FilesystemRead, true) }
	write := func() bool { return hostBool(opts.FilesystemWrite, true) }
	maxReadBytes := func() int64 {
		if opts.MaxFSReadBytes == nil {
			return 0
		}
		return opts.MaxFSReadBytes()
	}
	maxWriteBytes := func() int64 {
		if opts.MaxFSWriteBytes == nil {
			return 0
		}
		return opts.MaxFSWriteBytes()
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "fs." + name,
			Fn:   fn,
		}))
	}
	setRead := func(name string, fn func([]Value) ([]Value, error)) {
		set(name, func(args []Value) ([]Value, error) {
			if !read() {
				return nil, fmt.Errorf("filesystem read access disabled")
			}
			return fn(args)
		})
	}
	setWrite := func(name string, fn func([]Value) ([]Value, error)) {
		set(name, func(args []Value) ([]Value, error) {
			if !write() {
				return nil, fmt.Errorf("filesystem write access disabled")
			}
			return fn(args)
		})
	}
	setReadWrite := func(name string, fn func([]Value) ([]Value, error)) {
		set(name, func(args []Value) ([]Value, error) {
			if !read() {
				return nil, fmt.Errorf("filesystem read access disabled")
			}
			if !write() {
				return nil, fmt.Errorf("filesystem write access disabled")
			}
			return fn(args)
		})
	}
	checkReadSize := func(path string) error {
		max := maxReadBytes()
		if max <= 0 {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.Size() > max {
			return fmt.Errorf("filesystem read byte limit exceeded (%d)", max)
		}
		return nil
	}
	checkWriteSize := func(size int64) error {
		max := maxWriteBytes()
		if max <= 0 || size <= max {
			return nil
		}
		return fmt.Errorf("filesystem write byte limit exceeded (%d)", max)
	}

	// fs.exists(path) -> bool
	setRead("exists", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.exists' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{BoolValue(false)}, nil
		}
		_, err = os.Stat(p)
		return []Value{BoolValue(err == nil)}, nil
	})

	// fs.isfile(path) -> bool
	setRead("isfile", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.isfile' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{BoolValue(false)}, nil
		}
		info, err := os.Stat(p)
		if err != nil {
			return []Value{BoolValue(false)}, nil
		}
		return []Value{BoolValue(!info.IsDir())}, nil
	})

	// fs.isdir(path) -> bool
	setRead("isdir", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.isdir' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{BoolValue(false)}, nil
		}
		info, err := os.Stat(p)
		if err != nil {
			return []Value{BoolValue(false)}, nil
		}
		return []Value{BoolValue(info.IsDir())}, nil
	})

	// fs.stat(path) -> table or nil, errMsg
	setRead("stat", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.stat' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		info, err := os.Stat(p)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		stat := hostfs.ProjectFileInfo(info)
		tbl := NewTable()
		tbl.RawSet(StringValue("name"), StringValue(stat.Name))
		tbl.RawSet(StringValue("size"), IntValue(stat.Size))
		tbl.RawSet(StringValue("mtime"), FloatValue(float64(stat.MTime)))
		tbl.RawSet(StringValue("isdir"), BoolValue(stat.IsDir))
		tbl.RawSet(StringValue("isfile"), BoolValue(stat.IsFile()))
		tbl.RawSet(StringValue("mode"), StringValue(stat.Mode))
		return []Value{TableValue(tbl)}, nil
	})

	// fs.readfile(path) -> string or nil, errMsg
	setRead("readfile", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.readfile' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		if err := checkReadSize(p); err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(string(data))}, nil
	})

	// fs.writefile(path, content) -> true or nil, errMsg
	setWrite("writefile", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'fs.writefile' (path and content expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		if err := checkWriteSize(int64(len(args[1].Str()))); err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		err = os.WriteFile(p, []byte(args[1].Str()), 0644)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.appendfile(path, content) -> true or nil, errMsg
	setWrite("appendfile", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'fs.appendfile' (path and content expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		if err := checkWriteSize(int64(len(args[1].Str()))); err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		defer f.Close()
		_, err = f.WriteString(args[1].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.remove(path) -> true or nil, errMsg
	setWrite("remove", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.remove' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		err = os.Remove(p)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.removeAll(path) -> true or nil, errMsg
	setWrite("removeAll", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.removeAll' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		err = os.RemoveAll(p)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.rename(oldpath, newpath) -> true or nil, errMsg
	setWrite("rename", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'fs.rename' (oldpath and newpath expected)")
		}
		oldPath, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		newPath, err := resolveSandboxPath(root(), args[1].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		err = os.Rename(oldPath, newPath)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.mkdir(path) -> true or nil, errMsg
	setWrite("mkdir", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.mkdir' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		err = os.Mkdir(p, 0755)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.mkdirAll(path) -> true or nil, errMsg
	setWrite("mkdirAll", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.mkdirAll' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		err = os.MkdirAll(p, 0755)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.readdir(path) -> table (array of {name, isdir, size}) or nil, errMsg
	setRead("readdir", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.readdir' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		result := NewTable()
		for i, entry := range hostfs.ProjectDirEntries(entries) {
			entryTbl := NewTable()
			entryTbl.RawSet(StringValue("name"), StringValue(entry.Name))
			entryTbl.RawSet(StringValue("isdir"), BoolValue(entry.IsDir))
			entryTbl.RawSet(StringValue("size"), IntValue(entry.Size))
			result.RawSet(IntValue(int64(i+1)), TableValue(entryTbl))
		}
		return []Value{TableValue(result)}, nil
	})

	// fs.glob(pattern) -> table (array of matching paths) or nil, errMsg
	setRead("glob", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.glob' (string expected)")
		}
		pattern, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		result := NewTable()
		for i, m := range matches {
			result.RawSet(IntValue(int64(i+1)), StringValue(m))
		}
		return []Value{TableValue(result)}, nil
	})

	// fs.copy(src, dst) -> true or nil, errMsg
	setReadWrite("copy", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'fs.copy' (src and dst expected)")
		}
		srcPath := args[0].Str()
		srcPath, err := resolveSandboxPath(root(), srcPath)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		dstPath, err := resolveSandboxPath(root(), args[1].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		if err := checkReadSize(srcPath); err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		if maxWriteBytes() > 0 {
			info, err := os.Stat(srcPath)
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			if err := checkWriteSize(info.Size()); err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
		}

		srcFile, err := os.Open(srcPath)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	// fs.tempdir() -> string
	setRead("tempdir", func(args []Value) ([]Value, error) {
		if root() != "" {
			p, err := resolveSandboxPath(root(), ".")
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{StringValue(p)}, nil
		}
		return []Value{StringValue(os.TempDir())}, nil
	})

	// fs.tempfile([dir [, prefix]]) -> string (path) or nil, errMsg
	setWrite("tempfile", func(args []Value) ([]Value, error) {
		dir := ""
		prefix := ""
		if len(args) >= 1 && !args[0].IsNil() {
			dir = args[0].Str()
		}
		if root() != "" {
			if dir == "" {
				dir = "."
			}
			resolvedDir, err := resolveSandboxPath(root(), dir)
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			dir = resolvedDir
		}
		if len(args) >= 2 && !args[1].IsNil() {
			prefix = args[1].Str()
		}
		f, err := os.CreateTemp(dir, prefix)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		name := f.Name()
		f.Close()
		return []Value{StringValue(name)}, nil
	})

	// fs.cwd() -> string or nil, errMsg
	setRead("cwd", func(args []Value) ([]Value, error) {
		if root() != "" {
			p, err := resolveSandboxPath(root(), ".")
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{StringValue(p)}, nil
		}
		dir, err := os.Getwd()
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(dir)}, nil
	})

	// fs.chdir(path) -> true or nil, errMsg
	setWrite("chdir", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'fs.chdir' (string expected)")
		}
		p, err := resolveSandboxPath(root(), args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		err = os.Chdir(p)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{BoolValue(true)}, nil
	})

	return t
}
