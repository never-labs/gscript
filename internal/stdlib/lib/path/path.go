package path

import (
	"os"
	"path/filepath"
)

func Separator() string {
	return string(os.PathSeparator)
}

func ListSeparator() string {
	return string(os.PathListSeparator)
}

func Join(parts ...string) string {
	return filepath.Join(parts...)
}

func Dir(p string) string {
	return filepath.Dir(p)
}

func Base(p string) string {
	return filepath.Base(p)
}

func Ext(p string) string {
	return filepath.Ext(p)
}

func Abs(p string) (string, error) {
	return filepath.Abs(p)
}

func IsAbs(p string) bool {
	return filepath.IsAbs(p)
}

func Clean(p string) string {
	return filepath.Clean(p)
}

func Split(p string) (string, string) {
	return filepath.Split(p)
}

func Match(pattern, name string) (bool, error) {
	return filepath.Match(pattern, name)
}

func Rel(basepath, targpath string) (string, error) {
	return filepath.Rel(basepath, targpath)
}
