package archive

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"github.com/ulikunitz/xz"
	"os"
	"tag_highlight/util"
)

const ( // Compression Types
	COMP_NONE = iota
	COMP_GZIP
	COMP_BZIP2
	COMP_LZMA
)

//========================================================================================

func ReadFile(filename string, com_type int) [][]byte {
	timer := util.NewTimer()

	file, err := os.Open(filename)
	if err != nil {
		panic(fmt.Sprintf("os.Open error '%s'\n", err))
	}
	defer file.Close()
	var buf []byte

	switch com_type {
	case COMP_NONE:
		buf = read_plain(file)
	case COMP_GZIP:
		buf = read_gzip(file)
	case COMP_BZIP2:
		buf = read_bzip2(file)
	case COMP_LZMA:
		buf = read_lzma(file)
	default:
		panic(fmt.Sprintf("Illegal value %d passed to ReadFile.\n", com_type))
	}

	timer.EchoReport("writing file")
	return bytes.Split(buf, []byte("\n"))
}

//========================================================================================

func read_plain(file *os.File) []byte {
	var ret bytes.Buffer
	ret.ReadFrom(file)
	return ret.Bytes()
}

func read_gzip(file *os.File) []byte {
	var (
		reader   *gzip.Reader
		buf, ret bytes.Buffer
		err      error
	)

	buf.ReadFrom(file)

	if reader, err = gzip.NewReader(&buf); err != nil {
		panic(fmt.Sprintf("Decompression error %s", err))
	}
	ret.ReadFrom(reader)

	return ret.Bytes()
}

func read_bzip2(file *os.File) []byte {
	var buf, ret bytes.Buffer
	buf.ReadFrom(file)

	reader := bzip2.NewReader(&buf)
	ret.ReadFrom(reader)

	return ret.Bytes()
}

func read_lzma(file *os.File) []byte {
	var (
		reader   *xz.Reader
		buf, ret bytes.Buffer
		err      error
	)

	buf.ReadFrom(file)

	if reader, err = xz.NewReader(&buf); err != nil {
		panic(fmt.Sprintf("Decompression error %s", err))
	}
	ret.ReadFrom(reader)

	return ret.Bytes()
}
