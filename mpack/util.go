package mpack

import (
	"fmt"
	"os"
)

func eprintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func assert(cond bool, mes string, a ...interface{}) {
	if !cond {
		panic(fmt.Sprintf("Assertion failed: "+mes, a...))
	}
}
