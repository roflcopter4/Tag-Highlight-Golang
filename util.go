package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func Func() bstr {
	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	elems := strings.Split(fn.Name(), ".")
	return bstr(elems[len(elems)-1])
}

func Eprintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func panic_fmt(format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	panic(s)
}

func fsleep(length float64) {
	ilen := time.Duration(length * float64(time.Second))
	time.Sleep(ilen)
}

func max_int(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min_int(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func safe_fopen(fname string, mode, perm int) *os.File {
	file, e := os.OpenFile(fname, mode, os.FileMode(perm))
	if e != nil {
		panic(e)
	}
	return file
}

func safe_open(fname string, mode, perm int) int {
	fd, e := syscall.Open(fname, mode, uint32(perm))
	if e != nil {
		panic(e)
	}
	return fd
}

func assert(cond bool, mes string, a ...interface{}) {
	if !cond {
		panic(fmt.Sprintf(mes, a...))
	}
}
