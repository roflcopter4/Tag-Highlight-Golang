package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"tag_highlight/mpack"
)

type atomic_call struct {
	fmt  string
	args []interface{}
}
type atomic_list struct {
	calls []atomic_call
}

const STD_API_FMT string = "d,d,c:"
const (
	NW_STANDARD = iota
	NW_ERROR
	NW_ERROR_LN
)
const ( // Neovim mpack message types
	MES_REQUEST      = 0
	MES_RESPONSE     = 1
	MES_NOTIFICATION = 2
	MES_ANY          = 3
)

var (
	write_log       bool = false
	io_count        int  = 0
	sok_count       int  = 0
	main_nvim_mutex sync.Mutex
	decode_mutexes  map[int]*sync.Mutex
)

//========================================================================================

func Decode_Nvim_Stream(fd int, expect_mes_type int64) *mpack.Object {
	if fd == 1 {
		fd = 0
	}
	if decode_mutexes == nil {
		decode_mutexes = make(map[int]*sync.Mutex, 32)
	}
	mut := decode_mutexes[fd]
	if mut == nil {
		var tmp sync.Mutex
		decode_mutexes[fd] = &tmp
		mut = &tmp
	}
	mut.Lock()
	defer mut.Unlock()

	ret := mpack.Decode_Stream(fd)

	if ret.Mtype != mpack.T_ARRAY {
		log.Panic("For some reason neovim did not return an array.")
	}
	log_nvim_obj(ret, nil)

	ret_type := (ret.Index(0).Data.(int64))
	if expect_mes_type != MES_ANY && expect_mes_type != ret_type {
		switch ret_type {
		case MES_REQUEST:
			panic("This application cannot handle requests and neovim just sent one.")
		case MES_RESPONSE:
			Eprintf("Got unexpected response.\n")
			log_nvim_obj(ret, nil)
			return nil
		case MES_NOTIFICATION:
			panic("Not ready to accept notifications.")
		default:
			panic("Invalid neovim response.")
		}
	}

	return ret
}

//========================================================================================
// Main neovim api wrappers
//========================================================================================

func _do_call(log bool, fd, expect int, fn, format string, a []interface{}) interface{} {
	main_nvim_mutex.Lock()
	defer main_nvim_mutex.Unlock()
	write_log = log

	write_api(&fd, bstr(fn), format, a...)
	ret := Decode_Nvim_Stream(fd, MES_RESPONSE)
	return ret.Index(3).Expect(expect)
}

func generic_call(fd, expect int, fn, format string, a ...interface{}) interface{} {
	return _do_call(true, fd, expect, fn, format, a)
}

func nolog_call(fd, expect int, fn, format string, a ...interface{}) interface{} {
	return _do_call(false, fd, expect, fn, format, a)
}

func verify_only_call(fd int, fn, format string, a ...interface{}) error {
	main_nvim_mutex.Lock()
	defer main_nvim_mutex.Unlock()

	write_api(&fd, bstr(fn), format, a...)
	ret := Decode_Nvim_Stream(fd, MES_RESPONSE)

	if ret.Index(2).Mtype == mpack.T_ARRAY {
		return errors.New(
			ret.Index(2).Index(1).Expect(mpack.E_STRING).(string))
	}

	return nil
}

//----------------------------------------------------------------------------------------
// Message writing

func _nvim_write(fd, w_type int, mes bstr) {
	main_nvim_mutex.Lock()
	defer main_nvim_mutex.Unlock()
	check_def_fd(&fd)
	var fn bstr

	switch w_type {
	case NW_STANDARD:
		fn = bstr("nvim_out_write")
	case NW_ERROR:
		fn = bstr("nvim_err_write")
	case NW_ERROR_LN:
		fn = bstr("nvim_err_writeln")
	default:
		panic("Should not be reachable...")
	}

	pack := encode_fmt_api(fd, fn, "c", mes)
	write_pack(fd, pack)
	Decode_Nvim_Stream(fd, MES_RESPONSE)
}

func Nvim_printf(fd, w_type int, format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	_nvim_write(fd, w_type, bstr(str))
}

func echo(format string, a ...interface{}) {
	Nvim_printf(0, NW_STANDARD, format+"\n", a...)
}

//----------------------------------------------------------------------------------------
// Buffer functions

func Nvim_list_bufs(fd int) []int {
	fn := "nvim_list_bufs"
	return generic_call(fd, mpack.E_INTLIST, fn, "").([]int)
}

func Nvim_get_current_buf(fd int) int {
	fn := "nvim_get_current_buf"
	return int(generic_call(fd, mpack.T_NUM, fn, "").(int64))
}

func Nvim_buf_line_count(fd, bufnum int) int {
	fn := "nvim_buf_line_count"
	return int(generic_call(fd, mpack.T_NUM, fn, "d", bufnum).(int64))
}

func Nvim_buf_get_lines(fd, bufnum, start, end int) []string {
	fn := "nvim_buf_get_lines"
	ret := nolog_call(
		fd, mpack.E_STRLIST, fn, "d,d,d,B", bufnum, start, end, false).([]string)
	line := strings.Repeat("*", 120) + "\n"
	line += line

	// Logfiles["main"].WriteString(line + "INITIAL BUFFER UPDATE\n" + line + strings.Join(ret, "\n") + "\n" + line + "END UPDATE\n" + line)
	Logfiles["main"].WriteString(line + strings.Join(ret, "\n") + "\n" + line)
	return ret
}

func Nvim_buf_get_option(fd, bufnum int, optname string, expect int) interface{} {
	fn := "nvim_buf_get_option"
	return generic_call(fd, expect, fn, "d,s", bufnum, optname)
}

func Nvim_buf_get_name(fd, bufnum int) string {
	fn := "nvim_buf_get_name"
	fname := generic_call(fd, mpack.E_STRING, fn, "d", bufnum).(string)

	ret, e := filepath.Abs(fname)
	if e != nil {
		panic(e)
	}
	return ret
}

func Nvim_buf_get_changedtick(fd, bufnum int) int {
	fn := "nvim_buf_get_changedtick"
	return int(generic_call(fd, mpack.T_NUM, fn, "d", bufnum).(int64))
}

//----------------------------------------------------------------------------------------
// Vimscript commands and functions

func Nvim_command(fd int, cmd string) {
	fn := "nvim_command"
	e := verify_only_call(fd, fn, "s", cmd)
	if e != nil {
		log.Panicf("Nvim command failed with message '%s'", e)
	}
}

func Nvim_command_output(fd int, cmd string, expect int) interface{} {
	fn := "nvim_command_output"
	return generic_call(fd, expect, fn, "s", cmd)
}

func Nvim_call_function(fd int, function string, expect int) interface{} {
	fn := "nvim_call_function"
	return generic_call(fd, expect, fn, "s,[]", function)
}

//----------------------------------------------------------------------------------------
// Vim variables

func Nvim_get_var(fd int, varname string, expect int) interface{} {
	fn := "nvim_get_var"
	return generic_call(fd, expect, fn, "s", varname)
}

//----------------------------------------------------------------------------------------
// Misc

func Nvim_buf_attach(fd, bufnum int) {
	main_nvim_mutex.Lock()
	defer main_nvim_mutex.Unlock()
	fn := "nvim_buf_attach"

	// We don't wait for a response here
	// write_api(&fd, bstr(fn), "d,B,[]", bufnum, true)
	write_api(&fd, bstr(fn), "d,B,[]", bufnum, false)
}

// func Nvim_call_atomic(fd int, calls []atomic_call) error {
//         fn := "nvim_call_atomic"
//         fmt := STD_API_FMT
//         args := make([]interface{}, 0, 256)
//
//         args = append(args, MES_REQUEST, get_count(fd, true), bstr(fn))
//
//         if len(calls) > 0 {
//                 fmt += "[ [!" + calls[0].fmt + "],"
//                 args = append(args, calls[0].args)
//
//                 for i := 1; i < len(calls); i++ {
//                         fmt += "[!" + calls[i].fmt + "],"
//                         args = append(args, calls[i].args)
//                 }
//
//                 fmt += " ]:]"
//         }
//
//         main_nvim_mutex.Lock()
//         defer main_nvim_mutex.Unlock()
//
//         pack := mpack.Encode_fmt(uint(len(calls)), fmt, args...)
//
//         check_def_fd(&fd)
//         write_pack(fd, pack)
//
//         result := Decode_Nvim_Stream(fd, MES_RESPONSE)
//         if result.Index(2).Mtype != mpack.T_NIL {
//                 return errors.New("ERROR ERROR")
//         }
//
//         return nil
// }

func Nvim_call_atomic(fd int, call_list *atomic_list) error {
	fn := "nvim_call_atomic"
	fmt := STD_API_FMT + "[:"
	calls := call_list.calls
	// args := make([]interface{}, 0, 256)

	// args = append(args, MES_REQUEST, get_count(fd, true), bstr(fn))
	args := make([][]interface{}, 0, 128)

	if len(calls) > 0 {
		fmt += "[ @[" + calls[0].fmt + "],"
		args = append(args, calls[0].args)

		for i := 1; i < len(calls); i++ {
			fmt += "[*" + calls[i].fmt + "],"
			args = append(args, calls[i].args)
		}

		fmt += " ]:]"
	}

	main_nvim_mutex.Lock()
	defer main_nvim_mutex.Unlock()

	pack := mpack.Encode_fmt(uint(len(calls)), fmt, MES_REQUEST, get_count(fd, true), bstr(fn), &args)

	check_def_fd(&fd)
	write_pack(fd, pack)

	result := Decode_Nvim_Stream(fd, MES_RESPONSE)
	if result.Index(2).Mtype != mpack.T_NIL {
		return errors.New("ERROR ERROR")
	}

	return nil
}

//========================================================================================
// Helper functions
//========================================================================================

func check_def_fd(fd *int) {
	if *fd == 0 {
		*fd = Sockfd
	}
}

func write_api(fd *int, fn bstr, format string, a ...interface{}) {
	check_def_fd(fd)
	pack := encode_fmt_api(*fd, fn, format, a...)
	write_pack(*fd, pack)
}

func write_pack(fd int, pack *mpack.Object) {
	if fd == 0 {
		s := fmt.Sprintf("Cannot write to stdin!!! -> %d", fd)
		panic(s)
	}
	fmt.Fprintf(Logfiles["nvim"], "Writing request %d or %d.\n", io_count, sok_count)
	log_nvim_obj(pack, nil)
	syscall.Write(fd, pack.GetPack())
}

func get_count(fd int, inc bool) int {
	var ret int
	var cnt *int

	if fd == 1 {
		ret = io_count
		cnt = &io_count
	} else {
		ret = sok_count
		cnt = &sok_count
	}
	if inc {
		*cnt++
	}

	return ret
}

func encode_fmt_api(fd int, fn bstr, format string, a ...interface{}) *mpack.Object {
	b := make([]interface{}, 0, len(a)+3)
	b = append(b, MES_REQUEST, get_count(fd, true), fn)
	b = append(b, a...)
	return mpack.Encode_fmt(0, STD_API_FMT+"["+format+"]", b...)
}

func Nvim_get_var_fmt(fd, expect int, format string, a ...interface{}) interface{} {
	s := fmt.Sprintf(format, a...)
	return Nvim_get_var(fd, s, expect)
}

func log_nvim_obj(pack *mpack.Object, file *os.File) {
	ret_type := pack.Index(0).Get_Int()

	if mpack.DEBUG && write_log {
		if file == nil {
			switch ret_type {
			case MES_REQUEST:
				file = Logfiles["nvim"]
			case MES_RESPONSE:
				file = Logfiles["nvim"]
			case MES_NOTIFICATION:
				file = Logfiles["main"]
			}
		}

		if file != nil {
			file.Write(bstr("======================================\n"))
			pack.Print(file)
		}
	}

}
