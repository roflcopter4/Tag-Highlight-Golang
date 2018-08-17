package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"tag_highlight/archive"
	"tag_highlight/mpack"
	"time"
)

type bstr = []byte

type settings_t struct {
	Comp_type       uint16
	Comp_level      uint16
	Ctags_args      []string
	Enabled         bool
	Ignored_ftypes  []string
	Ignored_tags    map[string][]string
	Norecurse_dirs  []string
	Settings_file   string
	Use_compression bool
	Verbose         bool
}

var (
	Logfiles      = make(map[string]*os.File, 10)
	read_fd  int  = (-1)
	Sockfd   int  = (-1)
	DEBUG    bool = false
	HOME     string
	src_dir  string
	Settings settings_t
)

//========================================================================================

func main() {
	// mpack.DEBUG = true
	mpack.DEBUG = false
	DEBUG = true
	{
		var b bool
		HOME, b = os.LookupEnv("HOME")
		if !b {
			panic("Home directory not set.")
		}
		src_dir = HOME + "/go/src/tag_highlight"
	}
	logdir := src_dir + "/.logs"

	if e := syscall.Mkdir(logdir, 0755); e != nil && e != syscall.EEXIST {
		panic(e)
	}
	// Logfiles["nvim"] = safe_fopen(logdir+"/nvim.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// Logfiles["main"] = safe_fopen(logdir+"/main.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// Logfiles["cmds"] = safe_fopen(logdir+"/cms.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	// defer Logfiles["nvim"].Close()
	// defer Logfiles["main"].Close()
	// defer Logfiles["cmds"].Close()

	// read_fd = safe_open(HOME+"/decode_raw.log", os.O_RDONLY, 0644)
	read_fd = 0
	// defer syscall.Close(read_fd)

	Sockfd = create_socket()
	Eprintf("Created socket fd %d!\n", Sockfd)

	Settings = settings_t{
		Comp_type:       get_compression_type(0),
		Comp_level:      uint16(Nvim_get_var(0, pkg("compression_level"), mpack.T_NUM).(int64)),
		Ctags_args:      Nvim_get_var(0, pkg("ctags_args"), mpack.E_STRLIST).([]string),
		Enabled:         Nvim_get_var(0, pkg("enabled"), mpack.T_BOOL).(bool),
		Ignored_ftypes:  Nvim_get_var(0, pkg("ignore"), mpack.E_STRLIST).([]string),
		Ignored_tags:    Nvim_get_var(0, pkg("ignored_tags"), mpack.E_MAP_STR_STRLIST).(map[string][]string),
		Norecurse_dirs:  Nvim_get_var(0, pkg("norecurse_dirs"), mpack.E_STRLIST).([]string),
		Settings_file:   Nvim_get_var(0, pkg("settings_file"), mpack.E_STRING).(string),
		Use_compression: Nvim_get_var(0, pkg("use_compression"), mpack.T_BOOL).(bool),
		Verbose:         Nvim_get_var(0, pkg("verbose"), mpack.T_BOOL).(bool),
	}
	if !Settings.Enabled {
		os.Exit(0)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	var initial_buf int = (-1)

	tv1 := time.Now()
	// var tv1 syscall.Timeval

	for attempts := 0; buffers.mkr == 0; attempts++ {
		if attempts > 0 {
			if len(buffers.bad_bufs) > 0 {
				buffers.bad_bufs = []uint16{}
			}
			// fsleep(3.0)
			echo("Retrying initial connection (attempt %d)", attempts)
		}

		initial_buf = Nvim_get_current_buf(0)
		if New_Buffer(0, initial_buf) != nil {
			bdata := Find_Buffer(initial_buf)
			Nvim_buf_attach(1, initial_buf)

			bdata.get_initial_lines()
			bdata.Get_Initial_Taglist()
			bdata.Update_Highlight()

			// var tv2 syscall.Timespec
			// syscall.C

			tv2 := time.Now()
			// echo("Initial startup time: %f", (float64(tv1.))/float64(time.Second))
			// echo("Initial startup time: %s", time.Since(tv1).String())
			// syscall.Gettimeofday
			echo("%dns - %ds", tv2.Nanosecond(), tv2.Second())
			echo("Initial startup time: %f", tdiff(&tv1, &tv2))
		}
	}

	for {
		event := Decode_Nvim_Stream(1, MES_NOTIFICATION)
		if event != nil {
			event.Print(Logfiles["main"])
			handle_nvim_event(event)
		}
	}
}

func mpack_raw_str(bte []byte) string {
	if len(bte) == 0 {
		return ""
	}
	var s string
	for _, c := range bte {
		s += fmt.Sprintf("%02X ", c)
	}

	return s[:len(s)-1]
}

//========================================================================================

func main_loop(bufnum int) {
	sock := create_socket()

	{
		Nvim_buf_attach(sock, bufnum)
		ret1 := Decode_Nvim_Stream(sock, MES_ANY)
		ret2 := Decode_Nvim_Stream(sock, MES_ANY)
		var event *mpack.Object

		if ret1.Index(0).Expect(mpack.T_NUM).(int64) == MES_NOTIFICATION {
			event = ret1
		} else if ret2.Index(0).Expect(mpack.T_NUM).(int64) == MES_NOTIFICATION {
			event = ret2
		} else {
			panic("Didn't recieve an update from neovim...")
		}

		handle_nvim_event(event)
	}

	for {
		event := Decode_Nvim_Stream(sock, MES_NOTIFICATION)
		event.Print(Logfiles["main"])
		handle_nvim_event(event)
	}
}

//========================================================================================

func pkg(varname string) string {
	// This little monument to laziness should pretty much always get inlined.
	return "tag_highlight#" + varname
}

func create_socket() int {
	name := Nvim_call_function(1, "serverstart", mpack.E_STRING).(string)

	var addr syscall.SockaddrUnix
	addr.Name = name

	fd, e := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if e != nil {
		panic(e)
	}
	if e = syscall.Connect(fd, &addr); e != nil {
		panic(e)
	}

	return fd
}

func get_compression_type(fd int) uint16 {
	// tmp := Nvim_get_var(0, pkg("compression_type"), mpack.E_STRING).(string)
	var ret uint16 = archive.COMP_NONE

	// switch tmp {
	// case "gzip":
	//         ret = archive.COMP_GZIP
	// case "lzma":
	//         ret = archive.COMP_LZMA
	// case "none":
	//         ret = archive.COMP_NONE
	// default:
	//         echo("Warning: unrecognized compression type \"%s\", defaulting to no compression.", tmp)
	// }
	// echo("Compression type is '%s' -> %d", tmp, ret)

	return ret
}

func (bdata *Bufdata) get_initial_lines() {
	list := Nvim_buf_get_lines(0, int(bdata.Num), 0, (-1))
	if bdata.Lines.Qty == 1 {
		bdata.Lines.Delete_Node(bdata.Lines.Head)
	}
	bdata.Lines.Insert_Slice_After(bdata.Lines.Head, sl2i(list)...)
	bdata.Initialized = true
}

func tst() {
	calls := new(atomic_list)
	calls.nvim_command("echom 'Hello, moron!'")
	calls.nvim_command("echom 'Goodbye, moron!'")

	Nvim_call_atomic(0, calls)
}

func tdiff(tv1, tv2 *time.Time) float64 {
	return ((float64(tv2.Nanosecond()-tv1.Nanosecond()) / float64(1000000000.0)) +
		(float64(tv2.Second() - tv1.Second())))
}
