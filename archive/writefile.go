package archive

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/ulikunitz/xz"
	"os"
	"tag_highlight/api"
	"tag_highlight/util"
)

//========================================================================================

func WriteFile(filename string, data [][]byte, com_type int) bool {
	if data == nil {
		return false
	}
	var status bool
	timer := util.NewTimer()
	joined := bytes.Join(data, []byte("\n"))

	// api.Echo("Writing file '%s'", filename)

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("os.Create error '%s'\n", err))
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
		panic(fmt.Sprintf("Illegal value %d passed to WriteFile.\n", com_type))
	}

	timer.EchoReport("writing file")
	return status
}

//========================================================================================

func write_plain(file *os.File, data []byte) bool {
	if data == nil {
		return false
	}
	n, err := file.Write(data)

	if n != len(data) && err != nil {
		api.Echo("Warning: write error.\n")
		return false
	}

	return true
}

func write_gzip(file *os.File, data []byte) bool {
	writer := gzip.NewWriter(file)
	n, err := writer.Write(data)

	if n != len(data) || err != nil {
		api.Echo("Warning: gzip write error.\n")
		return false
	}

	return true
}

func write_bzip2(file *os.File, data []byte) bool {
	panic("Not implemented")
}

func write_lzma(file *os.File, data []byte) bool {
	if data == nil {
		return false
	}
	var (
		writer *xz.Writer
		err    error
		n      int
	)

	if writer, err = xz.NewWriter(file); err != nil {
		panic(fmt.Sprintf("Compression error %s", err))
	}
	defer writer.Close()

	n, err = writer.Write(data)
	if n != len(data) || err != nil {
		api.Echo("Warning: lzma write error.\n")
		return false
	}

	return true
}
