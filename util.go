package main

import (
	"bytes"
	"fmt"
	"log"
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

func warn(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "WARNING: "+format, a...)
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

func quick_read(filename string) []byte {
	// st, e := os.Stat(filename)
	// if e != nil {
	//         return nil
	// }
	//
	// var (
	//         ret  = make([]byte, 0, st.Size())
	//         file *os.File
	//         n    int
	// )

	var (
		buf  bytes.Buffer
		file *os.File
		e    error
	)

	if file, e = os.Open(filename); e != nil {
		return nil
	}
	/* if n, e = file.Read(ret); e != nil || int64(n) != st.Size() {
		log.Panicf("Unexpected io error: %s, (n=%d, size=%d)", e, n, st.Size())
	} */

	if _, e = buf.ReadFrom(file); e != nil {
		log.Panicf("Unexpected read error: %v\n", e)
	}

	file.Close()
	return buf.Bytes()
}

func unique_str(strlist []string) []string {
	keys := make(map[string]bool)
	ret := []string{}

	for _, entry := range strlist {
		if entry == "" {
			continue
		}
		if _, value := keys[entry]; !value {
			keys[entry] = true
			ret = append(ret, entry)
		}
	}

	return ret
}
