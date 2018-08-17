package archive

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"github.com/ulikunitz/xz"
	"log"
	"os"
	"strings"
)

const ( // Compression Types
	COMP_NONE = iota
	COMP_GZIP
	COMP_BZIP2
	COMP_LZMA
)

//========================================================================================

func ReadFile(filename string, com_type int) []string {
	file, err := os.Open(filename)
	if err != nil {
		log.Panicf("os.Open error '%s'\n", err)
	}
	defer file.Close()
	var buf string

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
		log.Panicf("Illegal value %d passed to ReadFile.\n", com_type)
	}

	return strings.Split(buf, "\n")
}

//========================================================================================

func read_plain(file *os.File) string {
	var ret bytes.Buffer
	ret.ReadFrom(file)
	return ret.String()
}

func read_gzip(file *os.File) string {
	var (
		reader   *gzip.Reader
		buf, ret bytes.Buffer
		err      error
	)

	buf.ReadFrom(file)

	if reader, err = gzip.NewReader(&buf); err != nil {
		log.Panicf("Decompression error %s", err)
	}
	ret.ReadFrom(reader)

	return ret.String()
}

func read_bzip2(file *os.File) string {
	var buf, ret bytes.Buffer
	buf.ReadFrom(file)

	reader := bzip2.NewReader(&buf)
	ret.ReadFrom(reader)

	return ret.String()
}

func read_lzma(file *os.File) string {
	var (
		reader   *xz.Reader
		buf, ret bytes.Buffer
		err      error
	)

	buf.ReadFrom(file)

	if reader, err = xz.NewReader(&buf); err != nil {
		log.Panicf("Decompression error %s", err)
	}
	ret.ReadFrom(reader)

	return ret.String()
}
