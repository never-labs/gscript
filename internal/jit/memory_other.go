//go:build !(darwin && arm64)

package jit

import "errors"

var errExecUnsupported = errors.New("jit: executable memory is only supported on darwin/arm64")

func AllocExec(size int) (*CodeBlock, error) {
	return nil, errExecUnsupported
}

func (b *CodeBlock) WriteCode(code []byte) error {
	return errExecUnsupported
}

func (b *CodeBlock) Free() error {
	return nil
}

func CallJIT(fn uintptr, ctx uintptr) int64 {
	return 0
}
