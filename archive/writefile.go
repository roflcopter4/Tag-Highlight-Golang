package archive

import (
	"compress/gzip"
	"github.com/ulikunitz/xz"
	"log"
	"os"
	"strings"
)

//========================================================================================

func WriteFile(filename string, data []string, com_type int) bool {
	if data == nil {
		return false
	}
	var status bool
	joined := strings.Join(data, "\n")

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("os.Create error '%s'\n", err)
	}
	defer file.Close()

	switch com_type {
	case COMP_NONE:
		status = write_plain(file, joined)
	case COMP_GZIP:
		status = write_gzip(file, joined)
	case COMP_BZIP2:
		status = write_bzip2(file, joined)
	case COMP_LZMA:
		status = write_lzma(file, joined)
	default:
		log.Fatalf("Illegal value %d passed to WriteFile.\n", com_type)
	}

	return status
}

//========================================================================================

func write_plain(file *os.File, data string) bool {
	if data == "" {
		return false
	}
	n, err := file.WriteString(data)

	if n != len(data) && err != nil {
		log.Printf("Warning: write error.\n")
		return false
	}

	return true
}

func write_gzip(file *os.File, data string) bool {
	var byte_data = []byte(data)

	writer := gzip.NewWriter(file)
	n, err := writer.Write(byte_data)

	if n != len(byte_data) || err != nil {
		log.Printf("Warning: gzip write error.\n")
		return false
	}

	return true
}

func write_bzip2(file *os.File, data string) bool {
	panic("Not implemented")
}

func write_lzma(file *os.File, data string) bool {
	if data == "" {
		return false
	}
	var (
		writer    *xz.Writer
		err       error
		n         int
		byte_data = []byte(data)
	)

	if writer, err = xz.NewWriter(file); err != nil {
		log.Fatalf("Compression error %s", err)
	}
	defer writer.Close()

	n, err = writer.Write(byte_data)
	if n != len(byte_data) || err != nil {
		log.Printf("Warning: lzma write error.\n")
		return false
	}

	return true
}
