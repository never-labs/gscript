#include "textflag.h"

// func getgptr() uintptr
//
// Returns the current goroutine's g pointer as an opaque identity token.
// On arm64 the g register is R28; the Go assembler exposes it as `g`.
TEXT ·getgptr(SB), NOSPLIT|NOFRAME, $0-8
	MOVD	g, R0
	MOVD	R0, ret+0(FP)
	RET
