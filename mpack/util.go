package mpack

import (
	"fmt"
	"os"
)

func eprintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func panic_fmt(format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	panic(s)
}
